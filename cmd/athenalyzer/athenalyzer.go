package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/athena"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
	"time"
)

var version = "0.0.1"

const awsRegion = "us-east-2"
const awsAthenaResultBucket = "personartb-aws-athena-query-results"
const awsAthenaDBName = "default"

const awsAthenaIDsQuery = `
	SELECT DISTINCT
			requestparameters
	FROM
			cloudtrail_logs_personartb_trails
	WHERE
			eventname='GetQueryExecution'
	AND
			eventtime > '%s'
	AND
			eventtime < '%s'
`

const awsAthenaBatchSize = 49
const durationSeconds = 2

func main() {
	argVersion := flag.Bool("version", false, "show version")
	argFromTime := flag.String("from-time", "", "from time (format: 0000-00-00T00:00:00Z)")
	argToTime := flag.String("to-time", "", "from time (format: 0000-00-00T00:00:00Z)")
	argAWSRegion := flag.String("aws-region", awsRegion, "set AWS Athena region")
	argOutputFile := flag.String("output-file", "", "set output file")
	argDebug := flag.Bool("debug", false, "turn on debug logging")
	argBiggerThen := flag.Int64("bigger-then", 1099511627776, "queries bigger then")
	argShowBigOnly := flag.Bool("big-only", false, "show big queries only")
	flag.Parse()

	logLevel := logrus.InfoLevel
	if *argDebug {
		logLevel = logrus.DebugLevel
	}
	logger := NewLogger(logLevel)

	if *argFromTime == "" || *argToTime == "" {
		logger.Errorf("from-time and to-time should not be empty")
		flag.Usage()
		os.Exit(31)

	}
	if *argOutputFile == "" {
		fileName := fmt.Sprintf("athenalyzer-result_%v-%v.csv", *argFromTime, *argToTime)
		fileName = strings.ReplaceAll(fileName, ":", "")
		argOutputFile = &fileName
	}

	if *argVersion {
		fmt.Println("Athenalyzer " + version)
		os.Exit(0)
	}

	stat := Stat{
		FromTime:   *argFromTime,
		ToTime:     *argToTime,
		ResultName: *argOutputFile,
		BiggerThen: *argBiggerThen,
	}
	logger.Debug(*argShowBigOnly)
	logger.Debug(stat.String())

	file, err := os.Create(*argOutputFile)
	defer file.Close()
	if err != nil {
		logger.Errorf("Can't create file %v", *argOutputFile)
		os.Exit(10)
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	athenaClient := athenaClient(argAWSRegion)
	query := fmt.Sprintf(awsAthenaIDsQuery, *argFromTime, *argToTime)
	//query := awsAthenaIDsQuery
	athenaRows, err := athenaQuery(athenaClient, awsAthenaDBName, query, awsAthenaResultBucket, logger)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ids := queryIDs(athenaRows)
	logger.Debugf("len of query ids: %v\n", len(ids))
	writer.Write([]string{
		"queryExecutionID",
		"Database",
		"SubmissionDateTime",
		"EngineExecutionTimeInMillis",
		"OutputLocation (bucket)",
		"Scanned(humanized)",
		"DataScannedInBytes",
		"Query",
	})

	var batch []*string
	for i, id := range ids {
		batch = append(batch, id)
		if (i%awsAthenaBatchSize == 0 && i != 0) || i == len(ids)-1 {
			qei := athena.BatchGetQueryExecutionInput{QueryExecutionIds: batch}
			output, err := athenaClient.BatchGetQueryExecution(&qei)
			logger.Debugf("len(batch)=%v len(output)=%v\n", len(batch), len(output.QueryExecutions))
			if err != nil {
				logger.Error(err)
				os.Exit(2)
			}

			for _, o := range output.QueryExecutions {
				stat.QueriesCount += 1
				stat.TotalDataBytes += *o.Statistics.DataScannedInBytes
				if *o.Statistics.DataScannedInBytes > stat.BiggerThen {
					stat.BigQueriesCount += 1
				}
				if *o.Statistics.DataScannedInBytes > stat.BiggestQueryBytes {
					stat.BiggestQueryBytes = *o.Statistics.DataScannedInBytes
				}
				row := []string{
					*o.QueryExecutionId,
					*o.QueryExecutionContext.Database,
					o.Status.SubmissionDateTime.String(),
				}

				et := int64(-1)
				if o.Statistics.EngineExecutionTimeInMillis != nil {
					et = *o.Statistics.EngineExecutionTimeInMillis
				}
				row = append(row, strconv.FormatInt(et, 10))
				row = append(row, strings.Split(*o.ResultConfiguration.OutputLocation, "/")[2])

				sb := int64(-1)
				if o.Statistics.DataScannedInBytes != nil {
					sb = *o.Statistics.DataScannedInBytes
				}
				row = append(row, humanize.Bytes(uint64(sb)))
				row = append(row, strconv.FormatInt(sb, 10))

				formatedQuery := strings.ReplaceAll(*o.Query, "\"", "'")
				formatedQuery = strings.ReplaceAll(formatedQuery, "\n", " ")
				formatedQuery = strings.ReplaceAll(formatedQuery, "\t", " ")
				row = append(row, formatedQuery)
				
				if *o.Statistics.DataScannedInBytes > stat.BiggestQueryBytes || ! *argShowBigOnly {
					err = writer.Write(row)
					if err != nil {
						logger.Errorf("Can't write file %v", err)
					}
				}
			}
			batch = nil
		}
	}

	fmt.Print(stat.String())
}

func athenaClient(awsRegion *string) *athena.Athena {
	awsCfg := &aws.Config{Region: awsRegion}
	awsSession := session.Must(session.NewSession(awsCfg))

	return athena.New(awsSession)
}

func athenaQuery(a *athena.Athena, db string, query string, bucketResult string, logger *logrus.Logger) ([]*athena.Row, error) {
	var s athena.StartQueryExecutionInput
	s.SetQueryString(query)

	var q athena.QueryExecutionContext
	q.SetDatabase(db)
	s.SetQueryExecutionContext(&q)

	var r athena.ResultConfiguration
	r.SetOutputLocation("s3://" + bucketResult)
	s.SetResultConfiguration(&r)

	result, err := a.StartQueryExecution(&s)
	if err != nil {
		return []*athena.Row{}, err
	}

	var qri athena.GetQueryExecutionInput
	qri.SetQueryExecutionId(*result.QueryExecutionId)

	var qrop *athena.GetQueryExecutionOutput
	duration := time.Duration(durationSeconds) * time.Second // Pause for 2 seconds

	for {
		qrop, err = a.GetQueryExecution(&qri)
		if err != nil {
			return []*athena.Row{}, err
		}

		time.Sleep(duration)
		logger.Debugf("athena query status: %v", *qrop.QueryExecution.Status.State)
		switch status := *qrop.QueryExecution.Status.State; status {
		case "QUEUED":
			continue
		case "RUNNING":
			continue
		case "SUCCEEDED":
			var ip athena.GetQueryResultsInput
			ip.SetQueryExecutionId(*result.QueryExecutionId)

			var rows []*athena.Row

			err := a.GetQueryResultsPages(&ip,
				func(page *athena.GetQueryResultsOutput, lastPage bool) bool {
					rows = append(rows, page.ResultSet.Rows...)
					if lastPage {
						return false
					}
					return true
				})
			if err != nil {
				return []*athena.Row{}, err
			}

			return rows[1:], nil
		}

		return []*athena.Row{}, errors.New("AWS returned:  unexpected status: " + *qrop.QueryExecution.Status.State)
	}
}

type queryExecutionID struct {
	QueryExecutionID string
}

func queryIDs(rows []*athena.Row) []*string {
	var ids []*string

	for _, row := range rows {
		var id queryExecutionID
		json.Unmarshal([]byte(*row.Data[0].VarCharValue), &id)
		ids = append(ids, &id.QueryExecutionID)
	}

	return ids
}

func NewLogger(level logrus.Level) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: false, FullTimestamp: true})
	logger.SetLevel(level)
	return logger
}

type Stat struct {
	FromTime          string
	ToTime            string
	TotalDataBytes    int64
	QueriesCount      int64
	BigQueriesCount   int64
	BiggerThen        int64
	ResultName        string
	BiggestQueryBytes int64
}

func (s *Stat) String() string {
	return fmt.Sprintf(
		"Period: %v - %v\nTotal data: %v\nCount of queries: %v\nCount of big queries (>%v): %v\nBiggest query: %v\nFile name: %v\n",
		s.FromTime,
		s.ToTime,
		humanize.Bytes(uint64(s.TotalDataBytes)),
		s.QueriesCount,
		humanize.Bytes(uint64(s.BiggerThen)),
		s.BigQueriesCount,
		humanize.Bytes(uint64(s.BiggestQueryBytes)),
		s.ResultName,
	)
}

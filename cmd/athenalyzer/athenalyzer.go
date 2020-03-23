package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/athena"
	"github.com/dustin/go-humanize"
	"os"
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
	flag.Parse()

	if *argVersion {
		fmt.Println("Athenalyzer " + version)
		os.Exit(0)
	}

	athenaClient := athenaClient(argAWSRegion)
	query := fmt.Sprintf(awsAthenaIDsQuery, *argFromTime, *argToTime)
	//query := awsAthenaIDsQuery
	athenaRows, err := athenaQuery(athenaClient, awsAthenaDBName, query, awsAthenaResultBucket)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ids := queryIDs(athenaRows)
	//fmt.Printf(" ###len(ids) = %v\n", len(ids))
	fmt.Print("queryExecutionID")
	fmt.Print(",")
	fmt.Print("Database")
	fmt.Print(",")
	fmt.Print("SubmissionDateTime")
	fmt.Print(",")
	fmt.Print("EngineExecutionTimeInMillis")
	fmt.Print(",")
	fmt.Print("OutputLocation (bucket)")
	fmt.Print(",")
	fmt.Print("Scanned(humanized)")
	fmt.Print(",")
	fmt.Print("DataScannedInBytes")
	fmt.Print(",")
	fmt.Print("Query")
	fmt.Println("")

	var batch []*string
	for i, id := range ids {
		batch = append(batch, id)
		if (i%awsAthenaBatchSize == 0 && i != 0) || i == len(ids)-1 {
			qei := athena.BatchGetQueryExecutionInput{QueryExecutionIds: batch}
			output, err := athenaClient.BatchGetQueryExecution(&qei)
			//fmt.Printf("### len(batch)=%v len(output)=%v\n", len(batch), len(output.QueryExecutions))
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}

			for _, o := range output.QueryExecutions {
				fmt.Printf("\"%v\"", *o.QueryExecutionId)
				fmt.Print(",")
				fmt.Printf("\"%v\"", *o.QueryExecutionContext.Database)
				fmt.Print(",")
				fmt.Printf("\"%v\"", *o.Status.SubmissionDateTime)
				fmt.Print(",")
				fmt.Printf("\"%v\"", *o.Statistics.EngineExecutionTimeInMillis)
				fmt.Print(",")
				fmt.Printf("\"%v\"", strings.Split(*o.ResultConfiguration.OutputLocation, "/")[2])
				fmt.Print(",")
				fmt.Printf("\"%v\"", humanize.Bytes(uint64(*o.Statistics.DataScannedInBytes)))
				fmt.Print(",")
				fmt.Printf("\"%v\"", *o.Statistics.DataScannedInBytes)
				fmt.Print(",")
				formatedQuery := strings.ReplaceAll(*o.Query, "\"", "'")
				formatedQuery = strings.ReplaceAll(formatedQuery, "\n", " ")
				formatedQuery = strings.ReplaceAll(formatedQuery, "\t", " ")
				fmt.Printf("\"%v\"", formatedQuery)
				fmt.Println("")
			}
			batch = nil
		}
	}
}

func athenaClient(awsRegion *string) *athena.Athena {
	awsCfg := &aws.Config{Region: awsRegion}
	awsSession := session.Must(session.NewSession(awsCfg))

	return athena.New(awsSession)
}

func athenaQuery(a *athena.Athena, db string, query string, bucketResult string) ([]*athena.Row, error) {
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

// TODO: в цикле бачами по 50 получать информацию о запросах по айдишникам и выводить в csv

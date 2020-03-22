# Athenalyzer

This application is designed to analyze the usage of the AWS Athena service.

## How it works:
1. Using AWS Athena API application send a request and get request id list.
```sql
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
```
2. With request id list application get information of each request (by Athena API).
3. Print output by CSV format.

How to use:
```bash

$ athenalyzer --help
Usage of athenalyzer:
  -aws-region string
    	set AWS Athena region (default "us-east-2")
  -from-time string
    	from time (format: 0000-00-00T00:00:00Z)
  -to-time string
    	from time (format: 0000-00-00T00:00:00Z)
  -version
    	show version
    	
$ athenalyzer --version
Athenalyzer 0.0.0-2-ge458357/2020-03-22_06:07:39


$ time athenalyzer -from-time 2020-03-21T00:00:00Z -to-time 2020-03-22T00:00:00Z > ./21032020.csv

real	6m3,490s
user	0m1,571s
sys	0m0,614s

```
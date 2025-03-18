package collector

import (
	"fmt"
	"time"
	"io"
	"bytes"
	"encoding/csv"
	"encoding/json"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/jszwec/csvutil"
)

type Job struct {
	ID            string
	User          string
	Status        string
	Queue         string
	FromHost      string
	ExecutionHost string
	JobName       string
	SubmitTime    int64
}

type JobCollector struct {
	JobInfo *prometheus.Desc
	logger  log.Logger
}

func init() {
	registerCollector("lsfjob", defaultEnabled, NewLSFJobCollector)
  fmt.Printf("%+s", "Init lsdjobs Called")
}

// NewLmstatCollector returns a new Collector exposing lmstat license stats.
func NewLSFJobCollector(logger log.Logger) (Collector, error) {

	return &JobCollector{
		JobInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "status"),
			"bjobs status labeled by id, user, status, queue and FromHost of the starttime.",
			[]string{"ID", "User", "Status", "Queue", "FromHost", "ExecutionHost", "JobName"}, nil,
		),
		logger: logger,
	}, nil
}

// Update calls c.getJobStatus to get the job info
// memory metrics.
func (c *JobCollector) Update(ch chan<- prometheus.Metric) error {

  fmt.Printf("%+s\n", "Update Called")
	err := c.getJobStatus(ch)
	if err != nil {
		return fmt.Errorf("couldn't get queues infomation: %w", err)
	}

	return nil
}

func parseJJobStatus() Job {
	now := time.Now().Unix()
	//level.Info(logger).Log("Parsing Job")
	return Job{
		ID:            "1",
		User:          "t01",
		Status:        "RUN",
		Queue:         "normal",
		FromHost:      "master01",
		ExecutionHost: "master01",
		JobName:       "sleep 10",
		SubmitTime:    now,
	}

}

type lsf_answer struct {
	COMMAND string `json:"COMMAND"`
  JOBS    int    `json:"JOBS"`
	RECORDS []bjobsInfo `json:"RECORDS"`
}

func bjobs_JsontoStruct(lsfOutput []byte, logger log.Logger) ([]bjobsInfo, error) {
	//var bjobsInfos []bjobsInfo
  lsfAnswer := &lsf_answer{}

  //fmt.Printf("%+s", lsfOutput)
	//err := json.Unmarshal(lsfOutput, &bjobsInfos)
	err := json.Unmarshal(lsfOutput, lsfAnswer)
	if err != nil {
    level.Error(logger).Log("err=", err)
    return nil, nil
  }
  //fmt.Printf("%+v\n", lsfAnswer.JOBS)
  //fmt.Printf("%+v\n", lsfAnswer.RECORDS)

	return lsfAnswer.RECORDS, nil
}

func bjobs_CsvtoStruct(lsfOutput []byte, logger log.Logger) ([]bjobsInfo, error) {
  csv_out := csv.NewReader(TrimReader{bytes.NewReader(lsfOutput)})
  csv_out.LazyQuotes = true
  csv_out.Comma = ' '
  csv_out.TrimLeadingSpace = true

  dec, err := csvutil.NewDecoder(csv_out)
  if err != nil {
    level.Error(logger).Log("err=", err)
    return nil, nil
  }

	var bjobsInfos []bjobsInfo

	for {
    var u bjobsInfo
    if err := dec.Decode(&u); err == io.EOF {
      break
    } else if err != nil {
      level.Error(logger).Log("err=", err)
      return nil, nil
    }

    bjobsInfos = append(bjobsInfos, u)
  }
  return bjobsInfos, nil

}

func (c *JobCollector) getJobStatus(ch chan<- prometheus.Metric) error {
  //output, err := lsfOutput(c.logger, "bjobs", "-w", "-u", "all")
	output, err := lsfOutput(c.logger, "bjobs", "-u", "all", "-o", "JOBID USER STAT SUBMIT_TIME", "-json")
	//output, err := lsfOutput(c.logger, "bjobs", "-u all -o 'JOBID USER STAT QUEUE EXEC_HOST NALLOC_SLOT MIN_REQ_PROC JOB_NAME SUBMIT_TIME START_TIME SUB_CWD' -json ")
	//output, err := lsfOutput(c.logger, "bjobs", "-u", "all", "-o", "'JOBID", "USER", "STAT", "QUEUE", "EXEC_HOST", "NALLOC_SLOT", "MIN_REQ_PROC", "JOB_NAME", "SUBMIT_TIME", "START_TIME", "SUB_CWD'", "-json")
	//output, err := lsfOutput(c.logger, "bjobs", "-u", "all", "-o", "'JOBID USER STAT QUEUE EXEC_HOST NALLOC_SLOT MIN_REQ_PROC JOB_NAME SUBMIT_TIME START_TIME SUB_CWD'", "-json")
  if err != nil {
    level.Error(c.logger).Log("err=", err, output)
    return nil
  }
  //fmt.Printf("%+s", output)
  //level.Info(c.logger).Log("err=", err, output)

	//jobs, err := bjobs_CsvtoStruct(output, c.logger)
	jobs, err := bjobs_JsontoStruct(output, c.logger)
	if err != nil {
    level.Error(c.logger).Log("err=", err)
    return nil
  }
  fmt.Printf("%+v\n", jobs)
  fmt.Printf("%+v\n", len(jobs))

	parseJobStatus := parseJJobStatus()
	ch <- prometheus.MustNewConstMetric(c.JobInfo, prometheus.GaugeValue, float64(parseJobStatus.SubmitTime), parseJobStatus.ID, parseJobStatus.User, parseJobStatus.Status, parseJobStatus.Queue, parseJobStatus.FromHost, parseJobStatus.ExecutionHost, parseJobStatus.JobName)
	return nil

}

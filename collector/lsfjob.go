package collector

import (
	"fmt"
//	"time"
	"io"
	"bytes"
	"strconv"
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
	SubmitTime    string
	UserGroup     string
	Project       string
	Application   string
	JGroup        string
	Dependency    string
	NSlot         string
  NProc         string
  StartTime     string
  SubCWD        string
	PendTime      string
	SrcJobid      string
  DstJobid      string
  SrcCluster    string
  DstCluster    string
}

type JobCollector struct {
	JobInfoNCpuCount *prometheus.Desc
  JobInfoPendingTime *prometheus.Desc
//	JobInfo *prometheus.Desc
	logger  log.Logger
}

func init() {
	registerCollector("lsfjob", defaultEnabled, NewLSFJobCollector)
  fmt.Printf("%+s", "Init lsdjobs Called")
}

// NewLSFJobCollector returns a new Collector exposing job info
func NewLSFJobCollector(logger log.Logger) (Collector, error) {

	return &JobCollector{
		JobInfoNCpuCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "ncpu_count"),
			"bjobs ncpu labeled by id, user, status, queue and FromHost of the starttime.",
			[]string{"ID", "User", "Status", "Queue", "FromHost", "ExecutionHost", "JobName",
		  "UserGroup", "Project", "Application", "JGroup", "Dependency", "NSlot", "NProc",
		  "StartTime", "SubCWD", "PendTime", "SrcJobid", "DstJobid", "SrcCluster", "DstCluster",
			"SubmitTime",
		}, nil,
		),

		JobInfoPendingTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bjobs", "pending_time_total"),
			"Job pending time since submission (sec)",
			[]string{"ID", "User", "Status", "Queue", "FromHost", "ExecutionHost", "JobName",
		  "UserGroup", "Project", "Application", "JGroup", "Dependency", "NSlot", "NProc",
		  "StartTime", "SubCWD", "PendTime", "SrcJobid", "DstJobid", "SrcCluster", "DstCluster",
			"SubmitTime",
		}, nil,
		),

		logger: logger,
	}, nil
}

// Update calls c.getJobStatus to get the job info
// memory metrics.
func (c *JobCollector) Update(ch chan<- prometheus.Metric) error {

  //fmt.Printf("%+s\n", "Update Called")
	err := c.getJobStatus(ch)
	if err != nil {
		return fmt.Errorf("couldn't get job infomation: %w", err)
	}

	return nil
}

func parseJobStatus( jobJson bjobsInfo ) Job {
	//now := time.Now().Unix()
	//level.Info(logger).Log("Parsing Job")

	//fmt.Printf("%s %+v\n", "SubmitTime:", jobJson.SUBMIT_TIME)
	//ts, err := time.Parse("Jan _2 15:04", jobJson.SUBMIT_TIME)
	//if err != nil {
	//	fmt.Printf("Error Decoding Time: %s\n", err)
  //}
	//ts = ts.AddDate(time.Now().Year(), 0, 0)

	return Job{
		ID:            jobJson.JOBID,
		User:          jobJson.USER,
		Status:        jobJson.STATUS,
		Queue:         jobJson.QUEUE,
		FromHost:      jobJson.FROM_HOST,
		ExecutionHost: jobJson.EXEC_HOST,
		JobName:       jobJson.JOB_NAME,
		UserGroup:     jobJson.UGROUP,
		Project:       jobJson.PROJECT,
		Application:   jobJson.APPLICATION,
		JGroup:        jobJson.JOB_GROUP,
		Dependency:    jobJson.DEPENDENCY,
	  NSlot:         jobJson.NALLOC_SLOT,
    NProc:         jobJson.MIN_REQ_PROC,
    StartTime:     jobJson.START_TIME,
    SubCWD:        jobJson.SUB_CWD,
		PendTime:      jobJson.PEND_TIME,
		SrcJobid:      jobJson.SRCJOBID,
    DstJobid:      jobJson.DSTJOBID,
    SrcCluster:    jobJson.SRCCLUSTER,
    DstCluster:    jobJson.DSTCLUSTER,
		SubmitTime:    jobJson.SUBMIT_TIME,   //int64(ts.Unix()),
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
	output, err := lsfOutput(c.logger, "bjobs", "-X", "-u", "all", "-o",
	"JOBID USER STAT QUEUE FROM_HOST EXEC_HOST JOB_NAME SUBMIT_TIME UGROUP PROJECT APPLICATION JOB_GROUP DEPENDENCY NALLOC_SLOT MIN_REQ_PROC START_TIME SUB_CWD PEND_TIME SRCJOBID DSTJOBID SRCLUSTER FWD_CLUSTER", "-json")
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
  //fmt.Printf("%+v\n", jobs)
  //fmt.Printf("%+v\n", len(jobs))

	for _, j := range jobs {

		jobStatus := parseJobStatus(j)
		fmt.Printf("%s %+v\n", "jobStatus:", jobStatus)
		nCPU, _ := strconv.ParseFloat(jobStatus.NProc, 64)
		// parameter order must follow declaration order of Labels (see top)
		ch <- prometheus.MustNewConstMetric(c.JobInfoNCpuCount, prometheus.GaugeValue, nCPU,
		  jobStatus.ID,
			jobStatus.User,
			jobStatus.Status,
			jobStatus.Queue,
			jobStatus.FromHost,
			jobStatus.ExecutionHost,
			jobStatus.JobName,
			jobStatus.UserGroup,
			jobStatus.Project,
			jobStatus.Application,
			jobStatus.JGroup,
			jobStatus.Dependency,
			jobStatus.NSlot,
			jobStatus.NProc,
			jobStatus.StartTime,
			jobStatus.SubCWD,
      jobStatus.PendTime,
			jobStatus.SrcJobid,
      jobStatus.DstJobid,
      jobStatus.SrcCluster,
      jobStatus.DstCluster,
			jobStatus.SubmitTime,
		)
		pTime, _ := strconv.ParseFloat(jobStatus.PendTime, 64)
		ch <- prometheus.MustNewConstMetric(c.JobInfoPendingTime, prometheus.CounterValue, pTime,
		  jobStatus.ID,
			jobStatus.User,
			jobStatus.Status,
			jobStatus.Queue,
			jobStatus.FromHost,
			jobStatus.ExecutionHost,
			jobStatus.JobName,
			jobStatus.UserGroup,
			jobStatus.Project,
			jobStatus.Application,
			jobStatus.JGroup,
			jobStatus.Dependency,
			jobStatus.NSlot,
			jobStatus.NProc,
			jobStatus.StartTime,
			jobStatus.SubCWD,
      jobStatus.PendTime,
			jobStatus.SrcJobid,
      jobStatus.DstJobid,
      jobStatus.SrcCluster,
      jobStatus.DstCluster,
			jobStatus.SubmitTime,
		)

		}

	return nil

}

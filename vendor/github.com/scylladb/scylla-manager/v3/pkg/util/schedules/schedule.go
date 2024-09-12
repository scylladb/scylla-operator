package schedules

import (
	"encoding/json"
	"time"

	"github.com/gocql/gocql"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

// Trigger provides the next activation date.
// Implementations must return the same values for the same now parameter.
// A zero time can be returned to indicate no more executions.
type Trigger interface {
	Next(now time.Time) time.Time
}

// Cron implements a trigger based on cron expression.
// It supports the extended syntax including @monthly, @weekly, @daily, @midnight, @hourly, @every <time.Duration>.
type Cron struct {
	CronSpecification
	inner Trigger
}

// CronSpecification combines specification for cron together with the start dates
// that defines the moment when the cron is being started.
type CronSpecification struct {
	Spec      string    `json:"spec"`
	StartDate time.Time `json:"start_date"`
}

func NewCron(spec string, startDate time.Time) (Cron, error) {
	t, err := NewCronTrigger(spec)
	if err != nil {
		return Cron{}, err
	}

	return Cron{
		CronSpecification: CronSpecification{
			Spec:      spec,
			StartDate: startDate,
		},
		inner: t,
	}, nil
}

func NewCronEvery(d time.Duration, startDate time.Time) Cron {
	c, _ := NewCron("@every "+d.String(), startDate) // nolint: errcheck
	return c
}

// MustCron calls NewCron and panics on error.
func MustCron(spec string, startDate time.Time) Cron {
	c, err := NewCron(spec, startDate)
	if err != nil {
		panic(err)
	}
	return c
}

// Next implements scheduler.Trigger.
func (c Cron) Next(now time.Time) time.Time {
	if c.inner == nil {
		return time.Time{}
	}
	if c.StartDate.After(now) {
		return c.inner.Next(c.StartDate)
	}
	return c.inner.Next(now)
}

func (c Cron) MarshalText() (text []byte, err error) {
	bytes, err := json.Marshal(c.CronSpecification)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot json marshal {%v}", c.CronSpecification)
	}
	return bytes, nil
}

func (c *Cron) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	var cronSpec CronSpecification
	err := json.Unmarshal(text, &cronSpec)
	if err != nil {
		// fallback to the < 3.2.6 approach where cron was not coupled with start date
		cronSpec = CronSpecification{
			Spec: string(text),
		}
	}

	if cronSpec.Spec == "" {
		return nil
	}
	v, err2 := NewCron(cronSpec.Spec, cronSpec.StartDate)
	if err2 != nil {
		return errors.Wrap(multierr.Combine(err, err2), "cron")
	}

	*c = v
	return nil
}

func (c Cron) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return nil, errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return c.MarshalText()
}

func (c *Cron) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	if i := info.Type(); i != gocql.TypeText && i != gocql.TypeVarchar {
		return errors.Errorf("invalid gocql type %s expected %s", info.Type(), gocql.TypeText)
	}
	return c.UnmarshalText(data)
}

func (c Cron) IsZero() bool {
	return c.inner == nil
}

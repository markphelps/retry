package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sleeperSpy struct {
	d time.Duration
}

func (s *sleeperSpy) Sleep(d time.Duration) {
	s.d = d
}

func TestRetriable_Do(t *testing.T) {
	type fields struct {
		maxAttempts uint
		maxWait     time.Duration
		waiter      Waiter
	}
	type want struct {
		retries      uint
		waitDuration time.Duration
		err          bool
	}
	tests := []struct {
		name   string
		fields fields
		f      func() error
		want   want
	}{
		{
			name: "exhaust retries",
			fields: fields{
				maxAttempts: 3,
				waiter:      Duration(time.Millisecond),
			},
			f: func() error {
				return errors.New("boom")
			},
			want: want{
				retries:      3,
				waitDuration: time.Millisecond,
				err:          true,
			},
		},
		{
			name: "permanent error",
			fields: fields{
				maxAttempts: 3,
				waiter:      Duration(time.Millisecond),
			},
			f: func() error {
				err := errors.New("boom")
				return Permanent(err)
			},
			want: want{
				retries: 0,
				err:     true,
			},
		},
		{
			name: "no error",
			fields: fields{
				maxAttempts: 3,
				waiter:      Duration(time.Millisecond),
			},
			f: func() error {
				return nil
			},
			want: want{
				retries: 0,
				err:     false,
			},
		},
		{
			name: "max wait - max wait wins",
			fields: fields{
				maxAttempts: 3,
				maxWait:     time.Millisecond,
				waiter:      Duration(time.Minute),
			},
			f: func() error {
				return errors.New("boom")
			},
			want: want{
				retries:      3,
				waitDuration: time.Millisecond,
				err:          true,
			},
		}, {
			name: "max wait - waiter wins",
			fields: fields{
				maxAttempts: 3,
				maxWait:     time.Minute,
				waiter:      Duration(time.Millisecond),
			},
			f: func() error {
				return errors.New("boom")
			},
			want: want{
				retries:      3,
				waitDuration: time.Millisecond,
				err:          true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sleeperSpy := &sleeperSpy{}
			sleep = sleeperSpy
			defer func() {
				sleep = defaultSleeper{}
			}()

			r := &Retriable{
				maxAttempts: tt.fields.maxAttempts,
				maxWait:     tt.fields.maxWait,
				waiter:      tt.fields.waiter,
			}
			err := r.Do(tt.f)
			if tt.want.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want.retries, r.retries)
			assert.Equal(t, tt.want.waitDuration, sleeperSpy.d)
		})
	}
}

func TestDuration(t *testing.T) {
	type inputs struct {
		attempt uint
		err     error
	}
	tests := []struct {
		name   string
		d      time.Duration
		inputs inputs
		want   time.Duration
	}{
		{
			name: "returns duration",
			d:    time.Second,
			inputs: inputs{
				attempt: 1,
			},
			want: time.Second,
		},
		{
			name: "returns duration regardless of attempt",
			d:    time.Second,
			inputs: inputs{
				attempt: 3,
			},
			want: time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Duration(tt.d)(tt.inputs.attempt, tt.inputs.err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestIncrementalBackoff(t *testing.T) {
	type inputs struct {
		attempt uint
		err     error
	}
	tests := []struct {
		name   string
		d      time.Duration
		inputs inputs
		want   time.Duration
	}{
		{
			name: "returns duration",
			d:    time.Second,
			inputs: inputs{
				attempt: 1,
			},
			want: time.Second,
		},
		{
			name: "returns duration with linear backoff",
			d:    time.Second,
			inputs: inputs{
				attempt: 3,
			},
			want: 3 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IncrementalBackoff(tt.d)(tt.inputs.attempt, tt.inputs.err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestExponentialBackoff(t *testing.T) {
	type inputs struct {
		attempt uint
		err     error
	}
	tests := []struct {
		name   string
		d      time.Duration
		inputs inputs
		want   time.Duration
	}{
		{
			name: "returns duration",
			d:    time.Second,
			inputs: inputs{
				attempt: 1,
			},
			want: time.Second,
		},
		{
			name: "returns duration with exponential backoff",
			d:    time.Second,
			inputs: inputs{
				attempt: 3,
			},
			want: 9 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExponentialBackoff(tt.d)(tt.inputs.attempt, tt.inputs.err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestDefault(t *testing.T) {
	got := Default()
	want := &Retriable{
		maxAttempts: 3,
		waiter:      Duration(time.Second),
	}
	assert.Equal(t, want.maxAttempts, got.maxAttempts)
	assert.ObjectsAreEqual(want.waiter, got.waiter)
}

func TestNew(t *testing.T) {
	type want struct {
		maxAttempts uint
		maxDuration time.Duration
		waiter      Waiter
	}
	tests := []struct {
		name string
		opts []Option
		want want
	}{
		{
			name: "default",
			want: want{
				maxAttempts: 3,
				waiter:      Duration(time.Second),
			},
		},
		{
			name: "with max attempts",
			opts: []Option{WithMaxAttempts(5)},
			want: want{
				maxAttempts: 5,
				waiter:      Duration(time.Second),
			},
		},
		{
			name: "with max wait",
			opts: []Option{WithMaxWait(5 * time.Second)},
			want: want{
				maxAttempts: 3,
				maxDuration: 5 * time.Second,
				waiter:      Duration(time.Second),
			},
		},
		{
			name: "with waiter",
			opts: []Option{WithWaiter(IncrementalBackoff(time.Second))},
			want: want{
				maxAttempts: 3,
				waiter:      IncrementalBackoff(time.Second),
			},
		},
		{
			name: "with max attempts and waiter",
			opts: []Option{WithMaxAttempts(5), WithWaiter(IncrementalBackoff(time.Second))},
			want: want{
				maxAttempts: 5,
				waiter:      IncrementalBackoff(time.Second),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.opts...)
			assert.Equal(t, tt.want.maxAttempts, got.maxAttempts)
			assert.Equal(t, tt.want.maxDuration, got.maxWait)
			assert.ObjectsAreEqual(tt.want.waiter, got.waiter)
		})
	}
}

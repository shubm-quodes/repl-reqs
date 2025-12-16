package network

import (
	"fmt"
	"net/http"
	"time"
)

// Poller keeps on polling upto 100 times until all mandatory conditions are satisfied
type Poller struct {
	req        *http.Request
	conditions []Condition
	delay      int
	maxRetries int
}

type result struct {
	resp *http.Response
	body any
}

func NewPoller(req *http.Request, c ...Condition) *Poller {
	return &Poller{
		req:        req,
		conditions: c,
		maxRetries: 100, // Default can be overriden
		delay:      500, // Default can be overriden
	}
}

func (p *Poller) SetDelay(d int) {
	p.delay = d
}

func (p *Poller) SetMaxRetries(m int) {
	p.maxRetries = m
}

func (p *Poller) Poll() (*http.Response, error) {
	for i := 0; i < p.maxRetries; i++ {
		res, err := p.executeAttempt()
		if err == nil && p.evaluateAll(res) {
			return res.resp, nil // Success!
		}

		time.Sleep(time.Duration(p.delay) * time.Millisecond)
	}

	return nil, fmt.Errorf("polling failed: conditions not met after %d retries", p.maxRetries)
}

// executeAttempt handles the HTTP roundtrip and one-time body parsing
func (p *Poller) executeAttempt() (*result, error) {
	client := &http.Client{Timeout: time.Duration(p.delay) * time.Millisecond}
	resp, err := client.Do(p.req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	helper := &BodyCondition{}
	body, _ := helper.getUnmarshalledBody(resp)

	return &result{resp: resp, body: body}, nil
}

func (p *Poller) evaluateAll(res *result) bool {
	if res == nil {
		return false
	}

	for _, cond := range p.conditions {
		if !p.check(cond, res) {
			return false
		}
	}
	return true
}

func (p *Poller) check(cond Condition, res *result) bool {
	if bCond, ok := cond.(*BodyCondition); ok {
		return bCond.evaluate(res.body)
	}

	return cond.Evaluate(res.resp)
}

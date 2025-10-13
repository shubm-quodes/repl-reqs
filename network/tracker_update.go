package network

import "net/http"

type Update struct {
	reqId string
	resp  *http.Response
	err error
}

func (u *Update) ReqId() string {
  return u.reqId
}

func (u *Update) Resp() *http.Response {
  return u.resp
}

func (u *Update) Err() error {
  return u.err
}

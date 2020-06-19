// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

type Mock struct {
	sync.RWMutex
	orig         ResponseStack
	stack        ResponseStack
	empty        *Response
	HttpLibError error
}

type Response struct {
	code int
	data []byte
	body interface{}
}

var ErrEmptyStack = errors.New("Empty stack")

func NewMockTransport() *Mock {
	return &Mock{
		orig:  ResponseStack{},
		stack: ResponseStack{},
	}
}

func (self *Mock) Append(code int, data []byte) {
	self.Lock()
	defer self.Unlock()
	self.orig.Put(&Response{code, data, nil})
	self.stack.Put(&Response{code, data, nil})
}

func (self *Mock) AppendWithBody(code int, body interface{}) {
	self.Lock()
	defer self.Unlock()
	self.orig.Put(&Response{code, []byte{}, body})
	self.stack.Put(&Response{code, []byte{}, body})
}

func (self *Mock) WhenEmpty(code int, data []byte) {
	self.Lock()
	defer self.Unlock()
	self.empty = &Response{code, data, nil}
}

func (self *Mock) RoundTrip(req *http.Request) (*http.Response, error) {
	if self.HttpLibError != nil {
		return nil, self.HttpLibError
	}

	self.Lock()
	defer self.Unlock()
	if self.stack.Empty() {
		if self.empty != nil {
			self.stack.Put(self.empty)
		} else {
			for _, v := range self.orig {
				self.stack.Put(v)
			}
		}
	}
	r := self.stack.Pop()
	resp := http.Response{
		StatusCode: r.code,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}
	if len(r.data) > 0 {
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(r.data))
	} else if r.body != nil {
		body := r.body.(io.ReadCloser)
		resp.Body = body
	} else {
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(nil))
	}
	return &resp, nil
}

// Response stack

type ResponseStack []*Response

func (s ResponseStack) Empty() bool      { return len(s) == 0 }
func (s ResponseStack) Peek() *Response  { return s[len(s)-1] }
func (s *ResponseStack) Put(v *Response) { *s = append(*s, v) }
func (s *ResponseStack) Pop() *Response {
	d := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return d
}

package service

import "github.com/stretchr/testify/mock"

type AlertClientMock struct {
	mock.Mock
}

func (a *AlertClientMock) Post(path string, body []byte) ([]byte, error) {
	args := a.Called(path, body)

	return args.Get(0).([]byte), args.Error(1)
}

func (a *AlertClientMock) ShouldPost(path string, body []byte, response []byte, err error) {
	a.
		On("Post", path, body).
		Once().
		Return(response, err)
}

func (a *AlertClientMock) Put(path string, body []byte) ([]byte, error) {
	args := a.Called(path, body)

	return args.Get(0).([]byte), args.Error(1)
}

func (a *AlertClientMock) ShouldPut(path string, body []byte, response []byte, err error) {
	a.
		On("Put", path, body).
		Once().
		Return(response, err)
}

func (a *AlertClientMock) Del(path string, body []byte) ([]byte, error) {
	args := a.Called(path, body)

	return args.Get(0).([]byte), args.Error(1)
}

func (a *AlertClientMock) ShouldDel(path string, body []byte, response []byte, err error) {
	a.
		On("Del", path, body).
		Once().
		Return(response, err)
}

func (a *AlertClientMock) Get(path string, body []byte) ([]byte, error) {
	args := a.Called(path, body)

	return args.Get(0).([]byte), args.Error(1)
}

func (a *AlertClientMock) ShouldGet(path string, body []byte, response []byte, err error) {
	a.
		On("Get", path, body).
		Once().
		Return(response, err)
}

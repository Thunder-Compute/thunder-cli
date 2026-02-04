package testutils

import (
	"net/http"
	"os"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return nil, nil
}

type SSHExecutor interface {
	Connect(host string, port int, tunnels []int) error
	Execute(command string) error
}

type MockSSHExecutor struct {
	ConnectFunc func(host string, port int, tunnels []int) error
	ExecuteFunc func(command string) error
}

func (m *MockSSHExecutor) Connect(host string, port int, tunnels []int) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(host, port, tunnels)
	}
	return nil
}

func (m *MockSSHExecutor) Execute(command string) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(command)
	}
	return nil
}

type SCPExecutor interface {
	Transfer(source, destination string) error
}

type MockSCPExecutor struct {
	TransferFunc func(source, destination string) error
}

func (m *MockSCPExecutor) Transfer(source, destination string) error {
	if m.TransferFunc != nil {
		return m.TransferFunc(source, destination)
	}
	return nil
}

type SSHConfigManager interface {
	RemoveInstance(hostAlias string) error
	AddInstance(hostAlias, hostname, user string, port int) error
}

type MockSSHConfigManager struct {
	RemoveInstanceFunc func(hostAlias string) error
	AddInstanceFunc    func(hostAlias, hostname, user string, port int) error
}

func (m *MockSSHConfigManager) RemoveInstance(hostAlias string) error {
	if m.RemoveInstanceFunc != nil {
		return m.RemoveInstanceFunc(hostAlias)
	}
	return nil
}

func (m *MockSSHConfigManager) AddInstance(hostAlias, hostname, user string, port int) error {
	if m.AddInstanceFunc != nil {
		return m.AddInstanceFunc(hostAlias, hostname, user, port)
	}
	return nil
}

type BrowserOpener interface {
	Open(url string) error
}

type MockBrowserOpener struct {
	OpenFunc func(url string) error
}

func (m *MockBrowserOpener) Open(url string) error {
	if m.OpenFunc != nil {
		return m.OpenFunc(url)
	}
	return nil
}

type FileSystem interface {
	ReadFile(filename string) ([]byte, error)
	WriteFile(filename string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
}

type MockFileSystem struct {
	ReadFileFunc  func(filename string) ([]byte, error)
	WriteFileFunc func(filename string, data []byte, perm os.FileMode) error
	MkdirAllFunc  func(path string, perm os.FileMode) error
	StatFunc      func(name string) (os.FileInfo, error)
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(filename)
	}
	return nil, nil
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(filename, data, perm)
	}
	return nil
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if m.MkdirAllFunc != nil {
		return m.MkdirAllFunc(path, perm)
	}
	return nil
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if m.StatFunc != nil {
		return m.StatFunc(name)
	}
	return nil, nil
}

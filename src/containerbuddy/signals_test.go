package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// Mock Discovery
type NoopDiscoveryService struct{}

func (c *NoopDiscoveryService) SendHeartbeat(service *ServiceConfig) {
	return
}

func (c *NoopDiscoveryService) CheckForUpstreamChanges(backend *BackendConfig) bool {
	return false
}

func (c *NoopDiscoveryService) MarkForMaintenance(service *ServiceConfig) {

}

func (c *NoopDiscoveryService) Deregister(service *ServiceConfig) {

}

// Note: We can only do handleSignals once
// or we'll start handling signals multiple times.
// So all the tests have to be done here.
func TestSignals(t *testing.T) {

	cmd := getCmd([]string{"/root/examples/test/test.sh", "interruptSleep"})
	service := &ServiceConfig{Name: "test-service", Poll: 1, discoveryService: &NoopDiscoveryService{}}
	config := &Config{Command: cmd, StopTimeout: 5, Services: []*ServiceConfig{service}}

	if inMaintenanceMode() {
		t.Errorf("Should not be in maintenance mode before starting handler")
	}
	handleSignals(config)
	if inMaintenanceMode() {
		t.Errorf("Should not be in maintenance mode after starting handler")
	}

	// Test SIGUSR1
	sendAndWaitForSignal(t, syscall.SIGUSR1)
	if !inMaintenanceMode() {
		t.Errorf("Should be in maintenance mode after receiving SIGUSR1")
	}
	sendAndWaitForSignal(t, syscall.SIGUSR1)
	if inMaintenanceMode() {
		t.Errorf("Should not be in maintenance mode after receiving second SIGUSR1")
	}

	// Test SIGTERM
	startTime := time.Now()
	quit := make(chan int, 1)
	go func() {
		if exitCode, _ := executeAndWait(cmd); exitCode != 2 {
			t.Errorf("Expected exit code 0 but got %d", exitCode)
		}
		quit <- 1
	}()
	runtime.Gosched()
	time.Sleep(10 * time.Millisecond)
	sendSignal(t, syscall.SIGTERM)
	<-quit
	close(quit)
	if elapsed := time.Since(startTime); elapsed.Seconds() > float64(config.StopTimeout) {
		t.Errorf("Expected elapsed time <= %d seconds, but was %.2f", config.StopTimeout, elapsed.Seconds())
	}
}

// helper to ensure that we block for the signal to deliver any signal
// we need, and then yield execution so that the handler gets a chance
// at running. If we don't do this there's a race where we can check
// resulting side-effects of a handler before it's been run

func sendSignal(t *testing.T, s os.Signal) {
	me, _ := os.FindProcess(os.Getpid())
	if err := me.Signal(s); err != nil {
		t.Errorf("Got error on %s: %v", s.String(), err)
	}
}

func sendAndWaitForSignal(t *testing.T, s os.Signal) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGUSR1)
	me, _ := os.FindProcess(os.Getpid())
	if err := me.Signal(s); err != nil {
		t.Errorf("Got error on SIGUSR1: %v", err)
	}
	<-sig
	runtime.Gosched()
}

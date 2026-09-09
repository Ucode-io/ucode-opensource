package v1

import (
	"testing"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestChannelEmitterDropsNonCriticalEventWhenFull(t *testing.T) {
	eventCh := make(chan SSEEvent, 1)
	eventCh <- SSEEvent{Type: EvProgress}
	emitter := &channelEmitter{ch: eventCh}

	emitter.Emit(SSEEvent{Type: EvProvider})

	if len(eventCh) != 1 {
		t.Fatalf("expected full channel to retain one event, got %d", len(eventCh))
	}
}

func TestChannelEmitterDeliversCriticalEventWhenFull(t *testing.T) {
	eventCh := make(chan SSEEvent, 1)
	eventCh <- SSEEvent{Type: EvProgress}
	emitter := &channelEmitter{ch: eventCh}
	sent := make(chan struct{})

	go func() {
		emitter.Emit(SSEEvent{Type: EvMobileProj})
		close(sent)
	}()

	select {
	case <-sent:
		t.Fatal("critical event must wait while channel is full")
	case <-time.After(20 * time.Millisecond):
	}

	<-eventCh
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("critical event was not delivered after channel had capacity")
	}

	if event := <-eventCh; event.Type != EvMobileProj {
		t.Fatalf("expected mobile project event, got %s", event.Type)
	}
}

func TestNewMobileProjectContract(t *testing.T) {
	generated := &models.GeneratedProject{
		ProjectName: "Pocket CRM",
		Files:       []models.ProjectFile{{Path: "App.tsx", Content: "app"}},
	}

	mobileProject := newMobileProject(generated, []models.MobileCapability{models.MobileCapabilityCamera})

	if mobileProject.ProjectType != mobileProjectType ||
		mobileProject.Runtime != mobileRuntime ||
		mobileProject.RuntimeVersion != capacitorRuntimeVersion ||
		mobileProject.WebDir != capacitorWebDir {
		t.Fatalf("unexpected mobile project metadata: %+v", mobileProject)
	}
	if len(mobileProject.Files) != 1 || mobileProject.Files[0].Path != "App.tsx" {
		t.Fatalf("unexpected mobile project files: %+v", mobileProject.Files)
	}
	if len(mobileProject.Capabilities) != 1 || mobileProject.Capabilities[0] != models.MobileCapabilityCamera {
		t.Fatalf("unexpected mobile project capabilities: %+v", mobileProject.Capabilities)
	}
}

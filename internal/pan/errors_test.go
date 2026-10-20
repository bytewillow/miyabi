package pan

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ppxb/miyabi/internal/domain"
)

func TestAPIErrorRateLimitClassifiesAsRateLimited(t *testing.T) {
	err := fmt.Errorf("ensure 115 movie directory: %w", &apiError{Code: RateLimitCode, Message: "已达到当前访问上限"})
	if kind := domain.KindOf(err); kind != domain.KindRateLimited {
		t.Fatalf("kind = %v, want %v", kind, domain.KindRateLimited)
	}
	delay, retry := domain.RetryDelay(err)
	if !retry || delay < 2*time.Minute {
		t.Fatalf("RetryDelay = (%v, %v), want a retry after at least 2m", delay, retry)
	}
}

func TestAPIErrorExposesPublicMessageWithoutLosingDiagnostics(t *testing.T) {
	want := "本视频正在云端转码中，请耐心等待"
	err := fmt.Errorf("get 115 playback URL: %w", &apiError{Code: 409, Message: want})
	var public interface{ PublicMessage() string }
	if !errors.As(err, &public) || public.PublicMessage() != want {
		t.Fatalf("public message = %#v, error = %v", public, err)
	}
	if !strings.Contains(err.Error(), "get 115 playback URL: 115 error 409:") {
		t.Fatalf("diagnostic context was lost: %v", err)
	}
	unauthorized := fmt.Errorf("read account: %w", &apiError{Code: 40140115, Message: "授权已过期"})
	if !errors.Is(unauthorized, ErrUnauthorized) || !errors.As(unauthorized, &public) || public.PublicMessage() != "授权已过期" {
		t.Fatal("public messages changed authorization error handling")
	}
}

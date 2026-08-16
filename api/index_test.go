package handler

import (
	"net/http/httptest"
	"testing"
)

func TestBuildTargetURLFromRewriteQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/api/index?path=android/app.apk&download=1", nil)

	target := buildTargetURL(req)

	want := "https://appdownload.modelcolorresearch.club/android/app.apk?download=1"
	if target.String() != want {
		t.Fatalf("unexpected target URL: got %q want %q", target.String(), want)
	}
}

func TestBuildTargetURLFromDirectPath(t *testing.T) {
	req := httptest.NewRequest("GET", "http://localhost/releases/client.dmg?channel=stable", nil)

	target := buildTargetURL(req)

	want := "https://appdownload.modelcolorresearch.club/releases/client.dmg?channel=stable"
	if target.String() != want {
		t.Fatalf("unexpected target URL: got %q want %q", target.String(), want)
	}
}

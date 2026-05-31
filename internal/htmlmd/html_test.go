package htmlmd

import (
	"strings"
	"testing"
)

func TestConvertConvertsImages(t *testing.T) {
	got, err := Convert(`<p>看图：</p><img src="/uploads/abc.png" alt="示例图">`)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	want := "![示例图](https://leetcode.cn/uploads/abc.png)"
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
}

func TestConvertKeepsAbsoluteImages(t *testing.T) {
	got, err := Convert(`<p><img src="https://assets.leetcode.cn/foo.png"></p>`)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	want := "![](https://assets.leetcode.cn/foo.png)"
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
}

func TestConvertCodeFence(t *testing.T) {
	got, err := Convert(`<pre><code class="language-c">int main() { return 0; }</code></pre>`)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	want := "```c\nint main() { return 0; }\n```"
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
}

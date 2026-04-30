package strutil

import (
	"strings"
	"testing"
)

func TestGenMediaObjectName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		width    int
		height   int
	}{
		{
			name:     "English image",
			filename: "photo.jpg",
		},
		{
			name:     "Chinese filename",
			filename: "我的照片.jpg",
		},
		{
			name:     "Special characters",
			filename: "photo@2024#version(1).png",
		},
		{
			name:     "Long filename",
			filename: "this_is_a_very_long_filename_with_many_characters_that_should_be_truncated.jpg",
		},
		{
			name:     "No extension",
			filename: "readme",
		},
		{
			name:     "Mixed case",
			filename: "MixedCase_Photo.PNG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenMediaObjectName(tt.filename, tt.width, tt.height)

			// 验证路径格式
			parts := strings.Split(result, "/")
			if len(parts) != 5 {
				t.Errorf("Expected 5 path parts, got %d: %s", len(parts), result)
			}

			// 验证media前缀
			if parts[0] != "media" {
				t.Errorf("Expected first part to be 'media', got '%s'", parts[0])
			}

			// 验证mediaType
			validTypes := map[string]bool{"image": true, "audio": true, "video": true, "common": true}
			if !validTypes[parts[1]] {
				t.Errorf("Invalid media type: %s", parts[1])
			}

			// 验证文件名长度 < 32（按字符计算）
			cleanedName := parts[4]
			cleanedRunes := []rune(cleanedName)
			if len(cleanedRunes) > 32 {
				t.Errorf("Cleaned filename length %d exceeds 32: %s", len(cleanedRunes), cleanedName)
			}

			t.Logf("Result: %s", result)
		})
	}
}

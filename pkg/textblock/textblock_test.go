package textblock

import (
	"reflect"
	"testing"
)

func IsErrorNil(err error) bool { return err == nil }

func Test_Read(t *testing.T) {
	tests := []struct {
		name           string
		data           string
		header         string
		footer         string
		want           []string
		isValidErrFunc func(err error) bool
	}{
		{
			name:           "no-blocks",
			data:           "something here\nsomething there",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "single-block",
			data:           "something here\n---- BEGIN ----\nhello world\n---- END ----\nsomething there",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"hello world"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "multiple-blocks",
			data:           "---- BEGIN ----\nhello world\n---- END ----\nsomething in the middle\n---- BEGIN ----\nhow are you today?\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"hello world", "how are you today?"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "multiline-single-block",
			data:           "---- BEGIN ----\nthis\n\tis\n\t\ta\n\t\t\ttest\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{"this\n\tis\n\t\ta\n\t\t\ttest"},
			isValidErrFunc: IsErrorNil,
		},
		{
			name:           "error-unexpected-start",
			data:           "---- BEGIN ----\nthis may\n---- BEGIN ----\nbe wrong\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorUnexpectedStart,
		},
		{
			name:           "error-unexpected-end",
			data:           "---- END ----\n---- BEGIN ----\nthis may\nbe wrong\n---- END ----",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorUnexpectedEnd,
		},
		{
			name:           "error-missing-end",
			data:           "---- BEGIN ----\nthis may\nbe wrong",
			header:         "---- BEGIN ----",
			footer:         "---- END ----",
			want:           []string{},
			isValidErrFunc: IsErrorMissingEnd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTextBlock(tt.header, tt.footer)
			blocks, err := parser.FindBlocks(tt.data)
			if !tt.isValidErrFunc(err) {
				t.Errorf("error got: %v", err)
			}
			if !reflect.DeepEqual(blocks, tt.want) {
				t.Errorf("got: %v, expected: %v", blocks, tt.want)
			}
		})
	}
}

func Test_Write(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		header string
		footer string
		want   string
	}{
		{
			name:   "empty-content",
			data:   "",
			header: "---- BEGIN ----",
			footer: "---- END ----",
			want:   "---- BEGIN ----\n---- END ----",
		},
		{
			name:   "multiline-content",
			data:   "hello\nworld",
			header: "---- BEGIN ----",
			footer: "---- END ----",
			want:   "---- BEGIN ----\nhello\nworld\n---- END ----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTextBlock(tt.header, tt.footer)
			output := parser.WriteBlock(tt.data)
			if !reflect.DeepEqual(output, tt.want) {
				t.Errorf("got: %v, expected: %v", output, tt.want)
			}
		})
	}
}

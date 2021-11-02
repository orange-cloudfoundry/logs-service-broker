package fakes

type FakeWriter struct {
	Buffer *string
}

func NewFakeWriter() *FakeWriter {
	return &FakeWriter{
		Buffer: new(string),
	}
}

func (fw FakeWriter) Write(b []byte) (int, error) {
	*fw.Buffer = string(b)
	return len(*fw.Buffer), nil
}

func (fw FakeWriter) Close() error {
	return nil
}

func (fw FakeWriter) GetBuffer() *string {
	return fw.Buffer
}

package syslog

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/orange-cloudfoundry/logs-service-broker/utils"
)

const QueryInGzip = "in_gzip"

type HttpWriter struct {
	url    string
	inGzip bool
}

func HttpDial(addr string) (*HttpWriter, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	inGzip := false
	inGzipRaw := u.Query().Get(QueryInGzip)

	if inGzipRaw != "" {
		inGzip, err = strconv.ParseBool(inGzipRaw)
		if err != nil {
			inGzip = true
		}
	}
	// nolint:staticcheck
	u.Query().Del(QueryInGzip)

	return &HttpWriter{
		url:    u.String(),
		inGzip: inGzip,
	}, nil
}

func (t *HttpWriter) Write(b []byte) (int, error) {
	if t.inGzip {
		return len(b), t.writeGzip(b)
	}
	return len(b), t.writePlain(b)
}

func (t *HttpWriter) writeGzip(b []byte) error {
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	if _, err := gw.Write(b); err != nil {
		return err
	}
	if err := gw.Flush(); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return t.post("gzip", buf)
}

func (t *HttpWriter) writePlain(b []byte) error {
	return t.post("", bytes.NewBuffer(b))
}

func (t *HttpWriter) post(contentEncoding string, r io.Reader) error {
	req, err := http.NewRequest("POST", t.url, r)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "text/plain")

	if contentEncoding != "" {
		req.Header.Add("Content-Encoding", contentEncoding)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer utils.CloseAndLogError(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}

	return nil
}

func (t *HttpWriter) Close() error {
	return nil
}

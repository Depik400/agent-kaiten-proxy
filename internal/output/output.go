package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Depik400/agent-kaiten-proxy/internal/apperr"
)

func JSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return apperr.Wrap(apperr.CodeKaitenAPI, "encode json", err, nil)
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	if err != nil {
		return apperr.Wrap(apperr.CodeKaitenAPI, "write output", err, nil)
	}
	return nil
}

func Error(w io.Writer, err error) {
	app, ok := err.(*apperr.Error)
	if !ok {
		app = apperr.New(apperr.CodeKaitenAPI, err.Error(), nil)
	}
	_ = JSON(w, app)
}

func Text(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	if err != nil {
		return apperr.Wrap(apperr.CodeKaitenAPI, "write output", err, nil)
	}
	return nil
}

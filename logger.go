package reporter

import (
	"log"
	"os"
)

var (
	logFlags = log.Ldate | log.Lmsgprefix | log.Ltime

	InfoLog  = log.New(os.Stdout, "[INFO] ", logFlags)
	WarnLog  = log.New(os.Stdout, "[WARN] ", logFlags)
	ErrorLog = log.New(os.Stderr, "[ERROR] ", logFlags)
)

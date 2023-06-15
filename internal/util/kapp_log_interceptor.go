/*
 * Copyright 2023 Damian Peckett <damian@pecke.tt>.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
)

var (
	kappLogLine = regexp.MustCompile(`^[0-9]{1,2}:[0-9]{2}:[0-9]{2}(AM|PM):\s+(.*)$`)
)

type KappLogInterceptor struct {
	logger logr.Logger
	stderr bool
	b      bytes.Buffer
}

func NewKappLogInterceptor(logger logr.Logger, stderr bool) *KappLogInterceptor {
	return &KappLogInterceptor{
		logger: logger,
		stderr: stderr,
	}
}

func (l *KappLogInterceptor) Write(p []byte) (n int, err error) {
	l.b.Write(p)

	scanner := bufio.NewScanner(&l.b)
	for scanner.Scan() {
		str := strings.TrimSpace(scanner.Text())
		if str != "" {
			matches := kappLogLine.FindStringSubmatch(str)
			if matches == nil || len(matches) != 3 {
				if l.stderr {
					l.logger.Error(nil, str)
				}

				continue
			}

			logMessage := matches[2]
			if l.stderr {
				l.logger.Error(nil, logMessage)
			} else {
				l.logger.Info(logMessage)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		l.logger.Error(err, "Error reading logs")
	}

	l.b.Reset()
	return len(p), nil
}

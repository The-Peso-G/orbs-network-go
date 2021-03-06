// Copyright 2019 the orbs-network-go authors
// This file is part of the orbs-network-go library in the Orbs project.
//
// This source code is licensed under the MIT license found in the LICENSE file in the root directory of this source tree.
// The above notice should be included in all copies or substantial portions of the software.

package metric

import (
	"fmt"
	"github.com/orbs-network/scribe/log"
	"sync/atomic"
)

type Text struct {
	namedMetric
	value atomic.Value
}

type textExport struct {
	Name  string
	Value string
}

func newText(name string, defaultValue ...string) *Text {
	value := ""

	if len(defaultValue) == 1 {
		value = defaultValue[0]
	}

	res := &Text{
		namedMetric: namedMetric{name: name},
		value:       atomic.Value{},
	}
	res.value.Store(value)
	return res
}
func (t *Text) Export() exportedMetric {
	return textExport{
		t.name,
		t.value.Load().(string),
	}
}

func (t *Text) Update(value string) {
	t.value.Store(value)
}

func (t *Text) String() string {
	return fmt.Sprintf("metric %s: %s\n", t.name, t.value)
}

func (t *Text) Value() string {
	return t.value.Load().(string)
}

func (t textExport) LogRow() []*log.Field {
	return []*log.Field{
		log.String("metric", t.Name),
		log.String("metric-type", "text"),
		log.String("text", t.Value),
	}
}

func (t textExport) PrometheusRow() []*prometheusRow {
	return nil
}

func (t textExport) PrometheusType() string {
	return ""
}

func (t textExport) PrometheusName() string {
	return ""
}

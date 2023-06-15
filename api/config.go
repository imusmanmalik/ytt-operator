package api

import (
	"fmt"
	"os"

	"github.com/dpeckett/ytt-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	Scheme = runtime.NewScheme()
)

func init() {
	_ = v1alpha1.AddToScheme(Scheme)
}

func LoadConfig(path string) (*v1alpha1.Config, error) {
	decode := serializer.NewCodecFactory(Scheme).UniversalDeserializer().Decode

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	conf, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	switch conf := conf.(type) {
	case *v1alpha1.Config:
		return conf, nil
	default:
		return nil, fmt.Errorf("unexpected config type: %T", conf)
	}
}

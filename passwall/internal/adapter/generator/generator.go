package generator

import (
	"fmt"

	"passwall/internal/model"
)

// Generator 配置生成器接口
type Generator interface {
	// Generate 生成配置
	Generate(proxies []*model.Proxy) ([]byte, error)
	// Format 返回生成的配置格式
	Format() string
}

// GeneratorFactory 配置生成器工厂
type GeneratorFactory interface {
	// RegisterGenerator 注册配置生成器
	RegisterGenerator(format string, generator Generator)
	// GetGenerator 获取配置生成器
	GetGenerator(format string) (Generator, error)
}

// DefaultGeneratorFactory 默认配置生成器工厂实现
type DefaultGeneratorFactory struct {
	generators map[string]Generator
}

// NewGeneratorFactory 创建配置生成器工厂
func NewGeneratorFactory() GeneratorFactory {
	return &DefaultGeneratorFactory{
		generators: make(map[string]Generator),
	}
}

// RegisterGenerator 注册配置生成器
func (f *DefaultGeneratorFactory) RegisterGenerator(format string, generator Generator) {
	f.generators[format] = generator
}

// GetGenerator 获取配置生成器
func (f *DefaultGeneratorFactory) GetGenerator(format string) (Generator, error) {
	generator, exists := f.generators[format]
	if !exists {
		return nil, fmt.Errorf("generator not found for format: %s", format)
	}
	return generator, nil
}

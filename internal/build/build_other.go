//go:build !linux

package build

import "context"

// InitReexec não faz nada fora do Linux.
func InitReexec() bool { return false }

func (Buildah) Build(context.Context, Options) (string, error)            { return "", ErrUnsupported }
func (Buildah) Push(context.Context, PushOptions) (string, error)         { return "", ErrUnsupported }
func (Buildah) Manifest(context.Context, ManifestOptions) (string, error) { return "", ErrUnsupported }

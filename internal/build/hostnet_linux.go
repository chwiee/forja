//go:build linux

package build

import (
	"errors"

	nettypes "go.podman.io/common/libnetwork/types"
)

// errNoNetworkBackend aparece se algo tentar criar uma rede isolada.
var errNoNetworkBackend = errors.New("forja usa só a rede do host: redes isoladas (netavark) não estão disponíveis")

// hostNetwork implementa a interface de rede que o buildah exige.
//
// Sem ela, o buildah procura o binário netavark ao criar cada container de
// build, mesmo quando os RUN usam a rede do host e o netavark nunca seria
// chamado. Como a imagem distroless não tem netavark, entregamos esta
// implementação que só responde "não há redes".
type hostNetwork struct{}

var _ nettypes.ContainerNetwork = hostNetwork{} // o compilador confere o contrato

func (hostNetwork) NetworkCreate(nettypes.Network, *nettypes.NetworkCreateOptions) (nettypes.Network, error) {
	return nettypes.Network{}, errNoNetworkBackend
}
func (hostNetwork) NetworkUpdate(string, nettypes.NetworkUpdateOptions) error {
	return errNoNetworkBackend
}
func (hostNetwork) NetworkRemove(string) error { return errNoNetworkBackend }
func (hostNetwork) NetworkList(...nettypes.FilterFunc) ([]nettypes.Network, error) {
	return nil, nil
}
func (hostNetwork) NetworkInspect(string) (nettypes.Network, error) {
	return nettypes.Network{}, errNoNetworkBackend
}
func (hostNetwork) Setup(string, nettypes.SetupOptions) (map[string]nettypes.StatusBlock, error) {
	return nil, errNoNetworkBackend
}
func (hostNetwork) Teardown(string, nettypes.TeardownOptions) error { return nil }
func (hostNetwork) RunInRootlessNetns(func() error) error           { return errNoNetworkBackend }
func (hostNetwork) RootlessNetnsInfo() (*nettypes.RootlessNetnsInfo, error) {
	return nil, errNoNetworkBackend
}
func (hostNetwork) PestoSocketPath() string           { return "" }
func (hostNetwork) Drivers() []string                 { return nil }
func (hostNetwork) DefaultNetworkName() string        { return "host" }
func (hostNetwork) NetworkInfo() nettypes.NetworkInfo { return nettypes.NetworkInfo{} }

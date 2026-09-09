package service

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"
)

// Error codes the HTTP layer still maps to a friendly message. Nothing raises
// them in the open-source build, but keeping them means the mapping does not
// have to be torn out too.
const (
	LimitCodeUsers    = "user_limit"
	LimitCodeBuilders = "builders_limit"
)

// Plan limits belong to the billing module, which is not part of the
// open-source build. The checks stay as call sites so the surrounding flow is
// unchanged, but a self-hosted installation has no plan to exceed and every
// check passes.
//
// The signatures keep their unused parameters on purpose: callers pass them
// today, and a distribution that reinstates billing only has to fill these two
// functions back in.

func checkUserProjectLimit(_ context.Context, _ client.ServiceManagerI, _ storage.StorageI, _, _ string) error {
	return nil
}

func checkUgenBuildersLimit(_ context.Context, _ client.ServiceManagerI, _ storage.StorageI, _, _ string) error {
	return nil
}

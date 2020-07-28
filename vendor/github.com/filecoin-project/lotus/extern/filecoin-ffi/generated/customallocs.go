package generated

/*
#cgo LDFLAGS: -L${SRCDIR}/.. -lfilcrypto
#cgo pkg-config: ${SRCDIR}/../filcrypto.pc
#include "../filcrypto.h"
#include <stdlib.h>
#include "cgo_helpers.h"
*/
import "C"
import (
	"unsafe"
)

// AllocateProxy allocates a FilPrivateReplicaInfo proxy object in the C heap,
// returning a function which, when called, frees the allocated memory. This
// method exists because the default c-for-go allocation strategy allocates a
// C struct with a field whose values is a pointer into the Go heap, which is
// not permitted by the most strict CGO check (cgocheck=2).
func (x *FilPrivateReplicaInfo) AllocateProxy() func() {
	mem := allocFilPrivateReplicaInfoMemory(1)
	proxy := (*C.fil_PrivateReplicaInfo)(mem)
	proxy.cache_dir_path = C.CString(x.CacheDirPath)
	proxy.comm_r = *(*[32]C.uint8_t)(unsafe.Pointer(&x.CommR))
	proxy.registered_proof = (C.fil_RegisteredPoStProof)(x.RegisteredProof)
	proxy.replica_path = C.CString(x.ReplicaPath)
	proxy.sector_id = (C.uint64_t)(x.SectorId)

	x.ref81a31e9b = proxy

	return func() {
		C.free(unsafe.Pointer(proxy.cache_dir_path))
		C.free(unsafe.Pointer(proxy.replica_path))
		C.free(unsafe.Pointer(proxy))
	}
}

// AllocateProxy allocates a FilPoStProof proxy object in the C heap,
// returning a function which, when called, frees the allocated memory.
func (x *FilPoStProof) AllocateProxy() func() {
	mem := allocFilPoStProofMemory(1)
	proxy := (*C.fil_PoStProof)(mem)

	proxy.registered_proof = (C.fil_RegisteredPoStProof)(x.RegisteredProof)
	proxy.proof_len = (C.size_t)(x.ProofLen)
	proxy.proof_ptr = (*C.uchar)(unsafe.Pointer(C.CString(x.ProofPtr)))

	x.ref3451bfa = proxy

	return func() {
		C.free(unsafe.Pointer(proxy.proof_ptr))
		C.free(unsafe.Pointer(proxy))
	}
}

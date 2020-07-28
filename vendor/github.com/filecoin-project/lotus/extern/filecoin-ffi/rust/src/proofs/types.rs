use std::io::{Error, SeekFrom};
use std::ptr;
use std::slice::from_raw_parts;

use anyhow::Result;
use drop_struct_macro_derive::DropStructMacro;
use ffi_toolkit::{code_and_message_impl, free_c_str, CodeAndMessage, FCPResponseStatus};
use filecoin_proofs_api::{
    PieceInfo, RegisteredPoStProof, RegisteredSealProof, UnpaddedBytesAmount,
};

#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub struct fil_32ByteArray {
    pub inner: [u8; 32],
}

/// FileDescriptorRef does not drop its file descriptor when it is dropped. Its
/// owner must manage the lifecycle of the file descriptor.
pub struct FileDescriptorRef(std::mem::ManuallyDrop<std::fs::File>);

impl FileDescriptorRef {
    #[cfg(not(target_os = "windows"))]
    pub unsafe fn new(raw: std::os::unix::io::RawFd) -> Self {
        use std::os::unix::io::FromRawFd;
        FileDescriptorRef(std::mem::ManuallyDrop::new(std::fs::File::from_raw_fd(raw)))
    }
}

impl std::io::Read for FileDescriptorRef {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        self.0.read(buf)
    }
}

impl std::io::Write for FileDescriptorRef {
    fn write(&mut self, buf: &[u8]) -> Result<usize, Error> {
        self.0.write(buf)
    }

    fn flush(&mut self) -> Result<(), Error> {
        self.0.flush()
    }
}

impl std::io::Seek for FileDescriptorRef {
    fn seek(&mut self, pos: SeekFrom) -> Result<u64, Error> {
        self.0.seek(pos)
    }
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub enum fil_RegisteredSealProof {
    StackedDrg2KiBV1,
    StackedDrg8MiBV1,
    StackedDrg512MiBV1,
    StackedDrg32GiBV1,
    StackedDrg64GiBV1,
}

impl From<RegisteredSealProof> for fil_RegisteredSealProof {
    fn from(other: RegisteredSealProof) -> Self {
        match other {
            RegisteredSealProof::StackedDrg2KiBV1 => fil_RegisteredSealProof::StackedDrg2KiBV1,
            RegisteredSealProof::StackedDrg8MiBV1 => fil_RegisteredSealProof::StackedDrg8MiBV1,
            RegisteredSealProof::StackedDrg512MiBV1 => fil_RegisteredSealProof::StackedDrg512MiBV1,
            RegisteredSealProof::StackedDrg32GiBV1 => fil_RegisteredSealProof::StackedDrg32GiBV1,
            RegisteredSealProof::StackedDrg64GiBV1 => fil_RegisteredSealProof::StackedDrg64GiBV1,
        }
    }
}

impl From<fil_RegisteredSealProof> for RegisteredSealProof {
    fn from(other: fil_RegisteredSealProof) -> Self {
        match other {
            fil_RegisteredSealProof::StackedDrg2KiBV1 => RegisteredSealProof::StackedDrg2KiBV1,
            fil_RegisteredSealProof::StackedDrg8MiBV1 => RegisteredSealProof::StackedDrg8MiBV1,
            fil_RegisteredSealProof::StackedDrg512MiBV1 => RegisteredSealProof::StackedDrg512MiBV1,
            fil_RegisteredSealProof::StackedDrg32GiBV1 => RegisteredSealProof::StackedDrg32GiBV1,
            fil_RegisteredSealProof::StackedDrg64GiBV1 => RegisteredSealProof::StackedDrg64GiBV1,
        }
    }
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
pub enum fil_RegisteredPoStProof {
    StackedDrgWinning2KiBV1,
    StackedDrgWinning8MiBV1,
    StackedDrgWinning512MiBV1,
    StackedDrgWinning32GiBV1,
    StackedDrgWinning64GiBV1,
    StackedDrgWindow2KiBV1,
    StackedDrgWindow8MiBV1,
    StackedDrgWindow512MiBV1,
    StackedDrgWindow32GiBV1,
    StackedDrgWindow64GiBV1,
}

impl From<RegisteredPoStProof> for fil_RegisteredPoStProof {
    fn from(other: RegisteredPoStProof) -> Self {
        use RegisteredPoStProof::*;

        match other {
            StackedDrgWinning2KiBV1 => fil_RegisteredPoStProof::StackedDrgWinning2KiBV1,
            StackedDrgWinning8MiBV1 => fil_RegisteredPoStProof::StackedDrgWinning8MiBV1,
            StackedDrgWinning512MiBV1 => fil_RegisteredPoStProof::StackedDrgWinning512MiBV1,
            StackedDrgWinning32GiBV1 => fil_RegisteredPoStProof::StackedDrgWinning32GiBV1,
            StackedDrgWinning64GiBV1 => fil_RegisteredPoStProof::StackedDrgWinning64GiBV1,
            StackedDrgWindow2KiBV1 => fil_RegisteredPoStProof::StackedDrgWindow2KiBV1,
            StackedDrgWindow8MiBV1 => fil_RegisteredPoStProof::StackedDrgWindow8MiBV1,
            StackedDrgWindow512MiBV1 => fil_RegisteredPoStProof::StackedDrgWindow512MiBV1,
            StackedDrgWindow32GiBV1 => fil_RegisteredPoStProof::StackedDrgWindow32GiBV1,
            StackedDrgWindow64GiBV1 => fil_RegisteredPoStProof::StackedDrgWindow64GiBV1,
        }
    }
}

impl From<fil_RegisteredPoStProof> for RegisteredPoStProof {
    fn from(other: fil_RegisteredPoStProof) -> Self {
        use RegisteredPoStProof::*;

        match other {
            fil_RegisteredPoStProof::StackedDrgWinning2KiBV1 => StackedDrgWinning2KiBV1,
            fil_RegisteredPoStProof::StackedDrgWinning8MiBV1 => StackedDrgWinning8MiBV1,
            fil_RegisteredPoStProof::StackedDrgWinning512MiBV1 => StackedDrgWinning512MiBV1,
            fil_RegisteredPoStProof::StackedDrgWinning32GiBV1 => StackedDrgWinning32GiBV1,
            fil_RegisteredPoStProof::StackedDrgWinning64GiBV1 => StackedDrgWinning64GiBV1,
            fil_RegisteredPoStProof::StackedDrgWindow2KiBV1 => StackedDrgWindow2KiBV1,
            fil_RegisteredPoStProof::StackedDrgWindow8MiBV1 => StackedDrgWindow8MiBV1,
            fil_RegisteredPoStProof::StackedDrgWindow512MiBV1 => StackedDrgWindow512MiBV1,
            fil_RegisteredPoStProof::StackedDrgWindow32GiBV1 => StackedDrgWindow32GiBV1,
            fil_RegisteredPoStProof::StackedDrgWindow64GiBV1 => StackedDrgWindow64GiBV1,
        }
    }
}

#[repr(C)]
#[derive(Clone)]
pub struct fil_PublicPieceInfo {
    pub num_bytes: u64,
    pub comm_p: [u8; 32],
}

impl From<fil_PublicPieceInfo> for PieceInfo {
    fn from(x: fil_PublicPieceInfo) -> Self {
        let fil_PublicPieceInfo { num_bytes, comm_p } = x;
        PieceInfo {
            commitment: comm_p,
            size: UnpaddedBytesAmount(num_bytes),
        }
    }
}

#[repr(C)]
#[derive(Clone)]
pub struct fil_PoStProof {
    pub registered_proof: fil_RegisteredPoStProof,
    pub proof_len: libc::size_t,
    pub proof_ptr: *const u8,
}

impl Drop for fil_PoStProof {
    fn drop(&mut self) {
        let _ = unsafe {
            Vec::from_raw_parts(self.proof_ptr as *mut u8, self.proof_len, self.proof_len)
        };
    }
}

#[derive(Clone, Debug)]
pub struct PoStProof {
    pub registered_proof: RegisteredPoStProof,
    pub proof: Vec<u8>,
}

impl From<fil_PoStProof> for PoStProof {
    fn from(other: fil_PoStProof) -> Self {
        let proof = unsafe { from_raw_parts(other.proof_ptr, other.proof_len).to_vec() };

        PoStProof {
            registered_proof: other.registered_proof.into(),
            proof,
        }
    }
}

#[repr(C)]
#[derive(Clone)]
pub struct fil_PrivateReplicaInfo {
    pub registered_proof: fil_RegisteredPoStProof,
    pub cache_dir_path: *const libc::c_char,
    pub comm_r: [u8; 32],
    pub replica_path: *const libc::c_char,
    pub sector_id: u64,
}

#[repr(C)]
#[derive(Clone)]
pub struct fil_PublicReplicaInfo {
    pub registered_proof: fil_RegisteredPoStProof,
    pub comm_r: [u8; 32],
    pub sector_id: u64,
}

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_GenerateWinningPoStSectorChallenge {
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
    pub ids_ptr: *const u64,
    pub ids_len: libc::size_t,
}

impl Default for fil_GenerateWinningPoStSectorChallenge {
    fn default() -> fil_GenerateWinningPoStSectorChallenge {
        fil_GenerateWinningPoStSectorChallenge {
            ids_len: 0,
            ids_ptr: ptr::null(),
            error_msg: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
        }
    }
}

code_and_message_impl!(fil_GenerateWinningPoStSectorChallenge);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_GenerateWinningPoStResponse {
    pub error_msg: *const libc::c_char,
    pub proofs_len: libc::size_t,
    pub proofs_ptr: *const fil_PoStProof,
    pub status_code: FCPResponseStatus,
}

impl Default for fil_GenerateWinningPoStResponse {
    fn default() -> fil_GenerateWinningPoStResponse {
        fil_GenerateWinningPoStResponse {
            error_msg: ptr::null(),
            proofs_len: 0,
            proofs_ptr: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
        }
    }
}

code_and_message_impl!(fil_GenerateWinningPoStResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_GenerateWindowPoStResponse {
    pub error_msg: *const libc::c_char,
    pub proofs_len: libc::size_t,
    pub proofs_ptr: *const fil_PoStProof,
    pub status_code: FCPResponseStatus,
}

impl Default for fil_GenerateWindowPoStResponse {
    fn default() -> fil_GenerateWindowPoStResponse {
        fil_GenerateWindowPoStResponse {
            error_msg: ptr::null(),
            proofs_len: 0,
            proofs_ptr: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
        }
    }
}

code_and_message_impl!(fil_GenerateWindowPoStResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_WriteWithAlignmentResponse {
    pub comm_p: [u8; 32],
    pub error_msg: *const libc::c_char,
    pub left_alignment_unpadded: u64,
    pub status_code: FCPResponseStatus,
    pub total_write_unpadded: u64,
}

impl Default for fil_WriteWithAlignmentResponse {
    fn default() -> fil_WriteWithAlignmentResponse {
        fil_WriteWithAlignmentResponse {
            comm_p: Default::default(),
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            left_alignment_unpadded: 0,
            total_write_unpadded: 0,
        }
    }
}

code_and_message_impl!(fil_WriteWithAlignmentResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_WriteWithoutAlignmentResponse {
    pub comm_p: [u8; 32],
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
    pub total_write_unpadded: u64,
}

impl Default for fil_WriteWithoutAlignmentResponse {
    fn default() -> fil_WriteWithoutAlignmentResponse {
        fil_WriteWithoutAlignmentResponse {
            comm_p: Default::default(),
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            total_write_unpadded: 0,
        }
    }
}

code_and_message_impl!(fil_WriteWithoutAlignmentResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_SealPreCommitPhase1Response {
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
    pub seal_pre_commit_phase1_output_ptr: *const u8,
    pub seal_pre_commit_phase1_output_len: libc::size_t,
}

impl Default for fil_SealPreCommitPhase1Response {
    fn default() -> fil_SealPreCommitPhase1Response {
        fil_SealPreCommitPhase1Response {
            error_msg: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
            seal_pre_commit_phase1_output_ptr: ptr::null(),
            seal_pre_commit_phase1_output_len: 0,
        }
    }
}

code_and_message_impl!(fil_SealPreCommitPhase1Response);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_FauxRepResponse {
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
    pub commitment: [u8; 32],
}

impl Default for fil_FauxRepResponse {
    fn default() -> fil_FauxRepResponse {
        fil_FauxRepResponse {
            error_msg: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
            commitment: Default::default(),
        }
    }
}

code_and_message_impl!(fil_FauxRepResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_SealPreCommitPhase2Response {
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
    pub registered_proof: fil_RegisteredSealProof,
    pub comm_d: [u8; 32],
    pub comm_r: [u8; 32],
}

impl Default for fil_SealPreCommitPhase2Response {
    fn default() -> fil_SealPreCommitPhase2Response {
        fil_SealPreCommitPhase2Response {
            error_msg: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
            registered_proof: fil_RegisteredSealProof::StackedDrg2KiBV1,
            comm_d: Default::default(),
            comm_r: Default::default(),
        }
    }
}

code_and_message_impl!(fil_SealPreCommitPhase2Response);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_SealCommitPhase1Response {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub seal_commit_phase1_output_ptr: *const u8,
    pub seal_commit_phase1_output_len: libc::size_t,
}

impl Default for fil_SealCommitPhase1Response {
    fn default() -> fil_SealCommitPhase1Response {
        fil_SealCommitPhase1Response {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            seal_commit_phase1_output_ptr: ptr::null(),
            seal_commit_phase1_output_len: 0,
        }
    }
}

code_and_message_impl!(fil_SealCommitPhase1Response);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_SealCommitPhase2Response {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub proof_ptr: *const u8,
    pub proof_len: libc::size_t,
}

impl Default for fil_SealCommitPhase2Response {
    fn default() -> fil_SealCommitPhase2Response {
        fil_SealCommitPhase2Response {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            proof_ptr: ptr::null(),
            proof_len: 0,
        }
    }
}

code_and_message_impl!(fil_SealCommitPhase2Response);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_UnsealRangeResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
}

impl Default for fil_UnsealRangeResponse {
    fn default() -> fil_UnsealRangeResponse {
        fil_UnsealRangeResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
        }
    }
}

code_and_message_impl!(fil_UnsealRangeResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_VerifySealResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub is_valid: bool,
}

impl Default for fil_VerifySealResponse {
    fn default() -> fil_VerifySealResponse {
        fil_VerifySealResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            is_valid: false,
        }
    }
}

code_and_message_impl!(fil_VerifySealResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_VerifyWinningPoStResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub is_valid: bool,
}

impl Default for fil_VerifyWinningPoStResponse {
    fn default() -> fil_VerifyWinningPoStResponse {
        fil_VerifyWinningPoStResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            is_valid: false,
        }
    }
}

code_and_message_impl!(fil_VerifyWinningPoStResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_VerifyWindowPoStResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub is_valid: bool,
}

impl Default for fil_VerifyWindowPoStResponse {
    fn default() -> fil_VerifyWindowPoStResponse {
        fil_VerifyWindowPoStResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            is_valid: false,
        }
    }
}

code_and_message_impl!(fil_VerifyWindowPoStResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_FinalizeTicketResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub ticket: [u8; 32],
}

impl Default for fil_FinalizeTicketResponse {
    fn default() -> Self {
        fil_FinalizeTicketResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            ticket: [0u8; 32],
        }
    }
}

code_and_message_impl!(fil_FinalizeTicketResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_GeneratePieceCommitmentResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub comm_p: [u8; 32],
    /// The number of unpadded bytes in the original piece plus any (unpadded)
    /// alignment bytes added to create a whole merkle tree.
    pub num_bytes_aligned: u64,
}

impl Default for fil_GeneratePieceCommitmentResponse {
    fn default() -> fil_GeneratePieceCommitmentResponse {
        fil_GeneratePieceCommitmentResponse {
            status_code: FCPResponseStatus::FCPNoError,
            comm_p: Default::default(),
            error_msg: ptr::null(),
            num_bytes_aligned: 0,
        }
    }
}

code_and_message_impl!(fil_GeneratePieceCommitmentResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_GenerateDataCommitmentResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub comm_d: [u8; 32],
}

impl Default for fil_GenerateDataCommitmentResponse {
    fn default() -> fil_GenerateDataCommitmentResponse {
        fil_GenerateDataCommitmentResponse {
            status_code: FCPResponseStatus::FCPNoError,
            comm_d: Default::default(),
            error_msg: ptr::null(),
        }
    }
}

code_and_message_impl!(fil_GenerateDataCommitmentResponse);

///

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_StringResponse {
    pub status_code: FCPResponseStatus,
    pub error_msg: *const libc::c_char,
    pub string_val: *const libc::c_char,
}

impl Default for fil_StringResponse {
    fn default() -> fil_StringResponse {
        fil_StringResponse {
            status_code: FCPResponseStatus::FCPNoError,
            error_msg: ptr::null(),
            string_val: ptr::null(),
        }
    }
}

code_and_message_impl!(fil_StringResponse);

#[repr(C)]
#[derive(DropStructMacro)]
pub struct fil_ClearCacheResponse {
    pub error_msg: *const libc::c_char,
    pub status_code: FCPResponseStatus,
}

impl Default for fil_ClearCacheResponse {
    fn default() -> fil_ClearCacheResponse {
        fil_ClearCacheResponse {
            error_msg: ptr::null(),
            status_code: FCPResponseStatus::FCPNoError,
        }
    }
}

code_and_message_impl!(fil_ClearCacheResponse);

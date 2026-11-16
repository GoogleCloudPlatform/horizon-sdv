<!-- Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Rust-to-C/C++ Foreign Function Interface (`extern "C"`) Boundary Safety Reference

This reference provides Rust templates, macros, and compile-time layout proofs for crossing `extern "C"` boundaries in ISO 26262 ASIL-B/D automotive software (security policy plugins, shared-memory IPC bridges, and packed binary wire headers).

---

## 1. Rule `FFI-ABI-001`: Unwind-Safe Panic Barrier Macro (`ffi_boundary!`)

### 1.1 Why Uncaught Panics Across `extern "C"` Are Catastrophic
In Rust, unwinding a panic across an `extern "C"` boundary into C or C++ caller frames is **immediate Undefined Behavior** (or forces an unconditional process abort in newer rustc editions).
Whenever a C ABI function returns a status enum or boolean, the panic barrier **must return a fail-safe error discriminant** rather than unwinding or aborting a shared multi-partition daemon.

### 1.2 Drop-In `ffi_boundary!` Macro
```rust
/// Wraps an `extern "C"` function body in `std::panic::catch_unwind`, converting
/// any internal Rust panic into a deterministic C fail-safe return code (`$err_ret`).
#[macro_export]
macro_rules! ffi_boundary {
    ($err_ret:expr, $body:block) => {{
        match ::std::panic::catch_unwind(::std::panic::AssertUnwindSafe(move || $body)) {
            Ok(val) => val,
            Err(_panic_payload) => {
                // Never unwind across extern "C" and never call process::abort() in a shared daemon
                $err_ret
            }
        }
    }};
}
```

---

## 2. Rule `FFI-ABI-002`: Defensive Pointer, Slice, and `CStr` Conversion Helpers

### 2.1 The `(NULL, 0)` Slice Trap in Rust vs. C/C++
In C and C++, a zero-length buffer represented as `(nullptr, 0)` is idiomatic.
However, in Rust, calling `core::slice::from_raw_parts(ptr, 0)` or `core::slice::from_raw_parts_mut(ptr, 0)` when `ptr` is `null` or unaligned is **immediate Undefined Behavior** (verified by `cargo miri` and the standard library precondition `data must be non-null and aligned even for zero-length slices`).

Use these four verified conversion helpers at every `extern "C"` boundary:

```rust
use core::ffi::CStr;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(i32)]
pub enum FfiPointerError {
    NullPointer = -1,
    UnalignedPointer = -2,
    LengthExceedsMaximum = -3,
    UnterminatedOrInvalidUtf8 = -4,
}

/// Safely constructs a shared `&'a [T]` slice from a C pointer and element count.
/// Handles `(NULL, 0)` safely by returning `&[]` without invoking `slice::from_raw_parts`.
///
/// # Safety
/// When `len > 0` and `ptr` is non-null and aligned, the caller must guarantee that `ptr`
/// points to `len` contiguous initialized elements of `T` valid for lifetime `'a`.
#[inline]
pub unsafe fn slice_from_c_parts<'a, T>(
    ptr: *const T,
    len: usize,
    max_elements: usize,
) -> Result<&'a [T], FfiPointerError> {
    if len > max_elements {
        return Err(FfiPointerError::LengthExceedsMaximum);
    }
    if len == 0 {
        return Ok(&[]);
    }
    if ptr.is_null() {
        return Err(FfiPointerError::NullPointer);
    }
    if !ptr.is_aligned() {
        return Err(FfiPointerError::UnalignedPointer);
    }
    if len.checked_mul(core::mem::size_of::<T>()).map_or(true, |b| b > isize::MAX as usize) {
        return Err(FfiPointerError::LengthExceedsMaximum);
    }
    // SAFETY: `len > 0`, `ptr` is non-null, aligned for `T`, and total byte size <= isize::MAX.
    Ok(unsafe { core::slice::from_raw_parts(ptr, len) })
}

/// Safely constructs a mutable `&'a mut [T]` slice from a C pointer and element count.
///
/// # Safety
/// When `len > 0` and `ptr` is non-null and aligned, the caller must guarantee that `ptr`
/// points to `len` contiguous initialized elements of `T` exclusively borrowed for `'a`.
#[inline]
pub unsafe fn slice_from_c_parts_mut<'a, T>(
    ptr: *mut T,
    len: usize,
    max_elements: usize,
) -> Result<&'a mut [T], FfiPointerError> {
    if len > max_elements {
        return Err(FfiPointerError::LengthExceedsMaximum);
    }
    if len == 0 {
        return Ok(&mut []);
    }
    if ptr.is_null() {
        return Err(FfiPointerError::NullPointer);
    }
    if !ptr.is_aligned() {
        return Err(FfiPointerError::UnalignedPointer);
    }
    if len.checked_mul(core::mem::size_of::<T>()).map_or(true, |b| b > isize::MAX as usize) {
        return Err(FfiPointerError::LengthExceedsMaximum);
    }
    // SAFETY: `len > 0`, `ptr` is non-null, aligned for `T`, and uniquely borrowed for `'a`.
    Ok(unsafe { core::slice::from_raw_parts_mut(ptr, len) })
}

/// Safely converts a raw `*const T` C pointer into `&'a T` after null and alignment validation.
///
/// # Safety
/// When `ptr` is non-null and aligned, it must point to a valid initialized `T` for `'a`.
#[inline]
pub unsafe fn ref_from_c_ptr<'a, T>(ptr: *const T) -> Result<&'a T, FfiPointerError> {
    if ptr.is_null() {
        return Err(FfiPointerError::NullPointer);
    }
    if !ptr.is_aligned() {
        return Err(FfiPointerError::UnalignedPointer);
    }
    // SAFETY: `ptr` is verified non-null and aligned to `align_of::<T>()`.
    Ok(unsafe { &*ptr })
}

/// Safely parses a bounded NUL-terminated C string (`const char*`), preventing unbounded `strlen`
/// reads on untrusted buffers.
///
/// # Safety
/// `ptr` must be readable up to the first NUL byte or `max_bytes`, whichever comes first.
#[inline]
pub unsafe fn cstr_from_ptr_bounded<'a>(
    ptr: *const core::ffi::c_char,
    max_bytes: usize,
) -> Result<&'a str, FfiPointerError> {
    if ptr.is_null() {
        return Err(FfiPointerError::NullPointer);
    }
    let byte_ptr = ptr.cast::<u8>();
    let mut nul_pos = None;
    for i in 0..max_bytes {
        // SAFETY: Caller guarantees readability up to first NUL or `max_bytes`.
        if unsafe { *byte_ptr.add(i) } == 0 {
            nul_pos = Some(i);
            break;
        }
    }
    let len_with_nul = nul_pos.ok_or(FfiPointerError::UnterminatedOrInvalidUtf8)? + 1;
    // SAFETY: `byte_ptr` is non-null and contains a NUL terminator at `len_with_nul - 1`.
    let bytes = unsafe { core::slice::from_raw_parts(byte_ptr, len_with_nul) };
    let cstr = CStr::from_bytes_with_nul(bytes).map_err(|_| FfiPointerError::UnterminatedOrInvalidUtf8)?;
    cstr.to_str().map_err(|_| FfiPointerError::UnterminatedOrInvalidUtf8)
}
```

---

## 3. Rule `FFI-ABI-003`: Layout-Verified `#[repr(C)]` Security Policy ABI

When a C/C++ host loads policy hooks via `dlsym`, annotate all shared structs and enums with `#[repr(C)]` and lock field sizes, alignments, and byte offsets at compile time:

```rust
use core::mem::{align_of, offset_of, size_of};

pub const SECURITY_POLICY_PORT_UNUSED: u16 = 0;
pub const SECURITY_POLICY_PORT_UNSET: u16 = 0xFFFF;

pub type security_policy_service_id_t = u16;
pub type security_policy_instance_id_t = u16;
pub type security_policy_member_id_t = u16;
pub type security_policy_ip_addr_t = u32;      // IPv4 address in network byte order
pub type security_policy_network_port_t = u16; // Port in network byte order
pub type uid_t = u32;
pub type gid_t = u32;

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct security_policy_client_t {
    pub user: uid_t,
    pub group: gid_t,
    pub host: security_policy_ip_addr_t,
    pub port: security_policy_network_port_t,
}

// Compile-time ABI layout proof matching C ABI on 32/64-bit POSIX targets
const _: () = {
    assert!(size_of::<security_policy_client_t>() == 16);
    assert!(align_of::<security_policy_client_t>() == 4);
    assert!(offset_of!(security_policy_client_t, user) == 0);
    assert!(offset_of!(security_policy_client_t, group) == 4);
    assert!(offset_of!(security_policy_client_t, host) == 8);
    assert!(offset_of!(security_policy_client_t, port) == 12);
};

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum security_policy_acl_result_t {
    SECURITY_POLICY_OK = 0,
    SECURITY_POLICY_PERM_DENIED = 1,
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum security_policy_result_t {
    SECURITY_POLICY_OK = 0,
    SECURITY_POLICY_NOT_FOUND = 1,
    SECURITY_POLICY_INVALID = 2,
}

/// Initializes the security policy plugin.
///
/// # Safety
/// Safe to call from multiple C/C++ threads; catches all internal Rust panics.
#[no_mangle]
pub unsafe extern "C" fn security_policy_initialize() -> security_policy_result_t {
    ffi_boundary!(security_policy_result_t::SECURITY_POLICY_INVALID, {
        security_policy_result_t::SECURITY_POLICY_OK
    })
}

/// Authenticates the connection with the local router.
///
/// # Safety
/// `router` must be either `NULL` or a valid, 4-byte aligned pointer to `security_policy_client_t`.
#[no_mangle]
pub unsafe extern "C" fn security_policy_authenticate_router(
    router: *const security_policy_client_t,
) -> security_policy_acl_result_t {
    ffi_boundary!(security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED, {
        // SAFETY: `ref_from_c_ptr` validates non-null and 4-byte alignment prior to dereference.
        let Ok(client) = (unsafe { ref_from_c_ptr(router) }) else {
            return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
        };
        if client.port != SECURITY_POLICY_PORT_UNUSED {
            return security_policy_acl_result_t::SECURITY_POLICY_OK;
        }
        if client.user != u32::MAX && client.group != u32::MAX {
            security_policy_acl_result_t::SECURITY_POLICY_OK
        } else {
            security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED
        }
    })
}

/// Checks whether `client` is allowed to access a service method or event.
///
/// # Safety
/// `client` must be either `NULL` or a valid, 4-byte aligned pointer to `security_policy_client_t`.
#[no_mangle]
pub unsafe extern "C" fn security_policy_is_client_allowed_to_access_member(
    client: *const security_policy_client_t,
    service: security_policy_service_id_t,
    instance: security_policy_instance_id_t,
    member: security_policy_member_id_t,
) -> security_policy_acl_result_t {
    ffi_boundary!(security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED, {
        // SAFETY: `ref_from_c_ptr` validates non-null and 4-byte alignment prior to dereference.
        let Ok(c) = (unsafe { ref_from_c_ptr(client) }) else {
            return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
        };
        if service == 0 || instance == 0 || member == 0xFFFF {
            return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
        }
        if c.port != SECURITY_POLICY_PORT_UNUSED || c.user != u32::MAX {
            security_policy_acl_result_t::SECURITY_POLICY_OK
        } else {
            security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED
        }
    })
}
```

---

## 4. Replacing `std::shared_ptr` Plugin Factories with a `#[repr(C)]` VTable

Returning `std::shared_ptr<Plugin>` across a `dlopen`/`dlsym` `extern "C"` boundary is a severe ABI hazard: `std::shared_ptr` has no C ABI stability guarantee across compilers, standard library versions (`libstdc++` vs `libc++`), or Rust crates, and its control block allocator belongs to the plugin `.so`.

Replace `std::shared_ptr<Plugin>` across `extern "C"` with an explicit `#[repr(C)]` vtable and opaque context pointer:

```rust
use core::ffi::{c_char, c_void};

#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AutomotivePluginType {
    ApplicationPlugin = 0,
    PreConfigurationPlugin = 1,
    ConfigurationPlugin = 2,
    RuntimeExtensionPlugin = 3,
}

#[repr(C)]
pub struct AutomotivePluginVTable {
    pub abi_version: u32,
    pub plugin_version: u32,
    pub plugin_type: AutomotivePluginType,
    pub plugin_name: *const c_char, // Static NUL-terminated ASCII string
    pub create: unsafe extern "C" fn() -> *mut c_void,
    pub destroy: unsafe extern "C" fn(ctx: *mut c_void),
}

// SAFETY: AutomotivePluginVTable contains only function pointers, scalars, and a static CStr pointer.
unsafe impl Sync for AutomotivePluginVTable {}
```

---

## 5. Rule `FFI-ABI-004`: Packed Binary Wire Headers (`#[repr(C, packed)]`)

When C headers declare wire structures with `__attribute__((aligned(1), packed))` and you mirror them in Rust with `#[repr(C, packed)]`:
1. **Never take references (`&hdr.field` or `&mut hdr.field`)** to multi-byte fields (`u16`, `u32`, `i32`).
2. Use `core::ptr::addr_of!(hdr.field).read_unaligned()` and `core::ptr::addr_of_mut!(hdr.field).write_unaligned(...)`, or parse directly from a byte slice `&[u8]` using `u16::from_be_bytes` / `u32::from_be_bytes`.

```rust
use core::mem::{align_of, offset_of, size_of};
use core::ptr::{addr_of, addr_of_mut};

pub const ECU_ID_SIZE: usize = 4;

#[repr(C, packed)]
#[derive(Copy, Clone)]
pub struct LoggingStandardHeader {
    pub htyp: u8,
    pub mcnt: u8,
    pub len: u16, // Big-endian total message length without storage header
}

#[repr(C, packed)]
#[derive(Copy, Clone)]
pub struct LogStorageHeader {
    pub pattern: [u8; ECU_ID_SIZE],
    pub seconds: u32,
    pub microseconds: i32,
    pub ecu: [u8; ECU_ID_SIZE],
}

const _: () = {
    assert!(size_of::<LoggingStandardHeader>() == 4);
    assert!(align_of::<LoggingStandardHeader>() == 1);
    assert!(offset_of!(LoggingStandardHeader, len) == 2);

    assert!(size_of::<LogStorageHeader>() == 16);
    assert!(align_of::<LogStorageHeader>() == 1);
    assert!(offset_of!(LogStorageHeader, seconds) == 4);
    assert!(offset_of!(LogStorageHeader, microseconds) == 8);
    assert!(offset_of!(LogStorageHeader, ecu) == 12);
};

impl LoggingStandardHeader {
    #[inline]
    pub fn message_length(&self) -> u16 {
        // SAFETY: `addr_of!` avoids creating an unaligned `&u16` reference on a packed struct.
        let raw_be = unsafe { addr_of!(self.len).read_unaligned() };
        u16::from_be(raw_be)
    }

    #[inline]
    pub fn set_message_length(&mut self, length_host: u16) {
        // SAFETY: `addr_of_mut!` writes a 2-byte big-endian integer at alignment 1 safely.
        unsafe { addr_of_mut!(self.len).write_unaligned(length_host.to_be()) };
    }
}
```

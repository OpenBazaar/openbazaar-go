# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Signing and verifying using [RSA-PSS](https://en.wikipedia.org/wiki/Probabilistic_signature_scheme).
- Signing and verifying using [Ed25519](https://ed25519.cr.yp.to/).
- `Audience` type for handling the `aud` claim [according to the RFC](https://tools.ietf.org/html/rfc7519#section-4.1.3).
- `Size` method to `Signer` interface.
- `Verifier` interface.
- `RawToken` type.
- SHA constants.

### Changed
- Improve performance by storing SHA hash functions in `sync.Pool`.
- Restructure `JWT` type by putting claims fields in a new struct.
- Change signing/verifying methods constructors' names.
- Unify signign/verifying methods constructors.
- Change `Signer` interface.
- Sign tokens with global function `Sign`.
- Verify tokens with global function `Verify`.

### Removed
- Support for `go1.10`.
- `Marshal` and `Unmarshal` functions.
- `Marshaler` and `Unmarshaler` interfaces.

## [2.0.0] - 2018-09-14
### Added
- `Parse` and `ParseBytes` functions.
- `Marshal` and `Unmarshal` functions.
- `Marshaler` interface.
- `Unmarshaler` interface.
- Content type header parameter.

### Changed
- Modify `Signer` signature.
- Add claims directly to `JWT` struct.
- Embed `header` to JWT.
- Add README texts, examples and usage.
- Rename `const.go` to `methods.go`.
- Add prefix `New` to signing methods constructors.
- Run `vgo` for testing (this enables testing the package against Go 1.10);

### Removed
- `Sign` and `Verify` functions.
- Base64 encoding and deconding functions.
- `Options` struct.
- `Claims` struct.
- Functions that extract JWT from contexts and requests.

## [1.1.0] - 2018-08-22
### Changed
- Prevent expensive slice reallocation when signing a JWT.
- Refactor tests.

### Fixed
- Signature of "none" algorithm.

### Removed
- `internal` package.

## [1.0.2] - 2018-07-19
### Removed
- Makefile.
- Benchmark test (unused).

## [1.0.1] - 2018-07-19
### Fixed
- Wrap Travis CI Golang versions in quotes (for parsing issues, see [this](https://github.com/travis-ci/travis-ci/issues/9247)).

## [1.0.0] - 2018-07-19
### Added
- AppVeyor configuration file for running tests in Windows.
- `vgo` module file.

### Changed
- `FromContext` now receives a context key as additional parameter.
- `FromContext` now tries to build a JWT if value in context is a string.
- Simplified Travis CI configuration file.
- Update README to explain the motivation to have created this library and its differences from other JWT libraries for Golang.

## [0.5.0] - 2018-03-12
### Added
- `FromContext` function to extract a JWT object from a context.
- `FromCookie` function to extract a JWT object from a cookie.

### Changed
- Split tests into several files in order to organize them.

### Fixed
- Example in README file.

## [0.4.0] - 2018-02-16
### Added
- Support for "none" method.
- Tests for "none" method.
- Missing JWTID claim.
- Plugable validation via validator functions.

### Changed
- `(*JWT).JWTID` method name to `(*JWT).ID`.

### Fixed
- Message in `ErrECDSASigLen`.

### Removed
- Comments from custom errors, since they are self-explanatory.

## [0.3.0] - 2018-02-13
### Changed
- Package structure.

### Removed
- Additional packages (`jwtcrypto` and `jwtutil`).

## [0.2.0] - 2018-02-06
### Added
- New test cases.
- Claims' timestamps validation.

### Changed
- Tests organization.
- Use `time.After` and `time.Before` for validating timestamps.
- `jwtcrypto/none.None` now implements `jwtcrypto.Signer`.

### Fixed
- Panicking when private or public keys are `nil`.

## 0.1.0 - 2018-02-06
### Added
- This changelog file.
- README file.
- MIT License.
- Travis CI configuration file.
- Makefile.
- Git ignore file.
- EditorConfig file.
- This package's source code, including examples and tests.
- Go dep files.

[Unreleased]: https://github.com/gbrlsnchs/jwt/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/gbrlsnchs/jwt/compare/v1.1.0...v2.0.0
[1.1.0]: https://github.com/gbrlsnchs/jwt/compare/v1.0.2...v1.1.0
[1.0.2]: https://github.com/gbrlsnchs/jwt/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/gbrlsnchs/jwt/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/gbrlsnchs/jwt/compare/v0.5.0...v1.0.0
[0.5.0]: https://github.com/gbrlsnchs/jwt/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/gbrlsnchs/jwt/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/gbrlsnchs/jwt/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/gbrlsnchs/jwt/compare/v0.1.0...v0.2.0

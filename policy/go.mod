module main

go 1.22

toolchain go1.22.2

require (
	github.com/google/go-tpm v0.9.1-0.20240411180339-1fb84445f623
	github.com/google/go-tpm-tools v0.3.13-0.20230620182252-4639ecce2aba
)

require (
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
)

// replace (
// 	github.com/google/go-tpm  => "./go-tpm"
// )

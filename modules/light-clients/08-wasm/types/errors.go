package types

import errorsmod "cosmossdk.io/errors"

var (
	ErrInvalid              = errorsmod.Register(ModuleName, 2, "invalid")
	ErrInvalidData          = errorsmod.Register(ModuleName, 3, "invalid data")
	ErrInvalidCodeHash      = errorsmod.Register(ModuleName, 4, "invalid code hash")
	ErrInvalidClientMessage = errorsmod.Register(ModuleName, 5, "invalid client message")
	ErrRetrieveClientID     = errorsmod.Register(ModuleName, 6, "failed to retrieve client id")
	ErrInvalidWasmClientID  = errorsmod.Register(ModuleName, 7, "invalid wasm client id")
	ErrClientNotFound       = errorsmod.Register(ModuleName, 8, "client not found")
	// Wasm specific
	ErrWasmEmptyCode             = errorsmod.Register(ModuleName, 9, "empty wasm code")
	ErrWasmCodeTooLarge          = errorsmod.Register(ModuleName, 10, "wasm code too large")
	ErrWasmCodeExists            = errorsmod.Register(ModuleName, 11, "wasm code already exists")
	ErrWasmCodeHashNotFound      = errorsmod.Register(ModuleName, 12, "wasm code hash not found")
	ErrWasmSubMessagesNotAllowed = errorsmod.Register(ModuleName, 13, "execution of sub messages is not allowed")
	ErrWasmEventsNotAllowed      = errorsmod.Register(ModuleName, 14, "returning events from a contract is not allowed")
	ErrWasmAttributesNotAllowed  = errorsmod.Register(ModuleName, 15, "returning attributes from a contract is not allowed")
	ErrWasmContractCallFailed    = errorsmod.Register(ModuleName, 16, "wasm contract call failed")
	ErrWasmInvalidResponseData   = errorsmod.Register(ModuleName, 17, "wasm contract returned invalid response data")
)
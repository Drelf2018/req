package req

import "unsafe"

type Any struct {
	Type  unsafe.Pointer
	Value unsafe.Pointer
}

func TypePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Type)
}

func ValuePtr(in any) uintptr {
	return uintptr((*Any)(unsafe.Pointer(&in)).Value)
}

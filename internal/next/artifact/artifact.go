package artifact

type Store interface {
	Read(cellPath string, artifact string, dest any) error
	Write(cellPath string, artifact string, src any) error
	Exists(cellPath string, artifact string) bool
}

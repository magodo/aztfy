package resourceid

type AzureResourceId interface {
	String() string
	TypeString() string
	Parent() AzureResourceId
	ParentScope() AzureResourceId
	Equal(AzureResourceId) bool
}

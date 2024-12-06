package resourceid

type MsGraphResourceId struct {
	id string
}

var _ AzureResourceId = &MsGraphResourceId{}

func NewMsGraphResourceId(id string) AzureResourceId {
	return &MsGraphResourceId{id: id}
}

func (m *MsGraphResourceId) Parent() AzureResourceId {
	return nil
}

func (m *MsGraphResourceId) ParentScope() AzureResourceId {
	return nil
}

func (m *MsGraphResourceId) String() string {
	return m.id
}

func (m *MsGraphResourceId) TypeString() string {
	return m.id
}

func (m *MsGraphResourceId) Equal(id AzureResourceId) bool {
	if m == nil || id == nil {
		return false
	}
	msgid, ok := id.(*MsGraphResourceId)
	if !ok {
		return false
	}
	return m.id == msgid.id
}

package resourceid

import "github.com/magodo/armid"

type ArmResourceId struct {
	id armid.ResourceId
}

var _ AzureResourceId = &ArmResourceId{}

func NewArmResourceId(id armid.ResourceId) AzureResourceId {
	return &ArmResourceId{id: id}
}

func (a *ArmResourceId) Parent() AzureResourceId {
	parent := a.id.Parent()
	if parent == nil {
		return nil
	}
	return &ArmResourceId{id: parent}
}

func (a *ArmResourceId) ParentScope() AzureResourceId {
	parentScope := a.id.ParentScope()
	if parentScope == nil {
		return nil
	}
	return &ArmResourceId{id: parentScope}
}

func (a *ArmResourceId) String() string {
	return a.id.String()
}

func (a *ArmResourceId) TypeString() string {
	return a.id.TypeString()
}

func (a *ArmResourceId) Equal(id AzureResourceId) bool {
	if a == nil || id == nil {
		return false
	}
	armid, ok := id.(*ArmResourceId)
	if !ok {
		return false
	}
	return a.id.Equal(armid.id)
}

package model

// GroupFaker 对应表 group_faker（群与马甲用户关联）
type GroupFaker struct {
	FakerId int `gorm:"column:faker_id;type:int(11);uniqueIndex:uniq_idx,priority:1" json:"faker_id"`
	GroupId int `gorm:"column:group_id;type:int(11);uniqueIndex:uniq_idx,priority:2" json:"group_id"`
}

func (GroupFaker) TableName() string {
	return "group_faker"
}

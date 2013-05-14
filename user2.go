
package main

type userView1 struct {
	Desc string
	Name string
}

type userV2 struct {
}

func loadUserV2() (m *userV2) {
	m = &userV2{}
	return
}

func (m *userV2) view1(name string) (view userView1) {
	if name == "test" {
		view.Desc = "测试用户"
	}
	return
}


package vm

import "testing"

func TestSort(t *testing.T) {
	in := []Instance{
		{Name: "b-lxc", Manager: "incus"},
		{Name: "z-vm", Manager: "truenas", IsVM: true},
		{Name: "a-vm", Manager: "truenas", IsVM: true},
		{Name: "m-vm", Manager: "incus", IsVM: true},
		{Name: "a-lxc", Manager: "incus"},
	}
	Sort(in)
	want := []string{"m-vm", "a-vm", "z-vm", "a-lxc", "b-lxc"}
	for i, n := range want {
		if in[i].Name != n {
			t.Fatalf("pos %d: got %s want %s (%v)", i, in[i].Name, n, in)
		}
	}
}

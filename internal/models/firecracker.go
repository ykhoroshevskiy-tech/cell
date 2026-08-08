package models

type BootSourceSpec struct {
	KernelImagePath string `json:"kernel_image_path"`
	BootArgs        string `json:"boot_args"`
}

type DriveSpec struct {
	DriveID      string `json:"drive_id"`
	PathOnHost   string `json:"path_on_host"`
	IsRootDevice bool   `json:"is_root_device"`
	IsReadOnly   bool   `json:"is_read_only"`
}

type MachineConfigSpec struct {
	VCPUCount   int    `json:"vcpu_count"`
	MemSizeMiB  int    `json:"mem_size_mib"`
	SMT         bool   `json:"smt"`
	CPUTemplate string `json:"cpu_template"`
}

type NetworkInterfaceSpec struct {
	IfaceID     string `json:"iface_id"`
	HostDevName string `json:"host_dev_name"`
	GuestMac    string `json:"guest_mac"`
}

type VmConfigDocument struct {
	BootSource        BootSourceSpec         `json:"boot_source"`
	Drives            []DriveSpec            `json:"drives"`
	MachineConfig     MachineConfigSpec      `json:"machine_config"`
	NetworkInterfaces []NetworkInterfaceSpec `json:"network_interfaces"`
}

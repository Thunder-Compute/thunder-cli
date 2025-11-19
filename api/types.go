package api

type Instance struct {
	ID        string `json:"-"`
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	IP        string `json:"ip"`
	CPUCores  string `json:"cpuCores"`
	Memory    string `json:"memory"`
	Storage   int    `json:"storage"`
	GPUType   string `json:"gpuType"`
	NumGPUs   string `json:"numGpus"`
	Mode      string `json:"mode"`
	Template  string `json:"template"`
	CreatedAt string `json:"createdAt"`
	Port      int    `json:"port"`
	K8s       bool   `json:"k8s"`
	Promoted  bool   `json:"promoted"`
}

type ThunderTemplateDefaultSpecs struct {
	Cores   int    `json:"cores"`
	GpuType string `json:"gpu_type"`
	NumGpus int    `json:"num_gpus"`
	Storage int    `json:"storage"`
}

type Template struct {
	Key                 string                      `json:"-"`
	DisplayName         string                      `json:"displayName"`
	ExtendedDescription string                      `json:"extendedDescription,omitempty"`
	AutomountFolders    []string                    `json:"automountFolders"`
	CleanupCommands     []string                    `json:"cleanupCommands"`
	OpenPorts           []int                       `json:"openPorts"`
	StartupCommands     []string                    `json:"startupCommands"`
	StartupMinutes      int                         `json:"startupMinutes,omitempty"`
	Version             int                         `json:"version,omitempty"`
	DefaultSpecs        ThunderTemplateDefaultSpecs `json:"defaultSpecs"`
}

type CreateInstanceRequest struct {
	CPUCores   int    `json:"cpu_cores"`
	GPUType    string `json:"gpu_type"`
	Template   string `json:"template"`
	NumGPUs    int    `json:"num_gpus"`
	DiskSizeGB int    `json:"disk_size_gb"`
	Mode       string `json:"mode"`
}

type CreateInstanceResponse struct {
	UUID       string `json:"uuid"`
	Message    string `json:"message"`
	Identifier int    `json:"identifier"`
	Key        string `json:"key"`
}

type DeleteInstanceResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type AddSSHKeyResponse struct {
	UUID string `json:"uuid"`
	Key  string `json:"key"`
}

type DeviceIDResponse struct {
	ID string `json:"id"`
}

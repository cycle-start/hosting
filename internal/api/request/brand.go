package request

type CreateBrand struct {
	ID               string `json:"id" validate:"required,slug"`
	Name             string `json:"name" validate:"required"`
	BaseHostname     string `json:"base_hostname" validate:"required"`
	PrimaryNS        string `json:"primary_ns" validate:"required"`
	SecondaryNS      string `json:"secondary_ns" validate:"required"`
	HostmasterEmail  string `json:"hostmaster_email" validate:"required"`
}

type UpdateBrand struct {
	Name             *string `json:"name"`
	BaseHostname     *string `json:"base_hostname"`
	PrimaryNS        *string `json:"primary_ns"`
	SecondaryNS      *string `json:"secondary_ns"`
	HostmasterEmail  *string `json:"hostmaster_email"`
}

type SetBrandClusters struct {
	ClusterIDs []string `json:"cluster_ids" validate:"required"`
}

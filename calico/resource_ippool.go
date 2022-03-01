package calico

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	calicov3 "github.com/projectcalico/api/pkg/apis/projectcalico/v3"
	clientset "github.com/projectcalico/api/pkg/client/clientset_generated/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilValidation "k8s.io/apimachinery/pkg/util/validation"
)

var defaultAttributes = map[string]interface{}{
	"block_size":         26,
	"ipip_mode":          "Never",
	"vxlan_mode":         "Never",
	"nat_outgoing":       false,
	"disabled":           false,
	"disable_bgp_export": false,
}

func resourceIPPool() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIPPoolCreate,
		ReadContext:   resourceIPPoolRead,
		UpdateContext: resourceIPPoolUpdate,
		DeleteContext: resourceIPPoolDelete,
		Schema:        resourceCalicoIPPoolSchemaV3(),
	}
}

func validateAnnotations(value interface{}, key string) (ws []string, es []error) {
	m := value.(map[string]interface{})
	for k := range m {
		errors := utilValidation.IsQualifiedName(strings.ToLower(k))
		if len(errors) > 0 {
			for _, e := range errors {
				es = append(es, fmt.Errorf("%s (%q) %s", key, k, e))
			}
		}
	}
	return
}

func resourceCalicoIPPoolSchemaV3() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"metadata": {
			Type:        schema.TypeList,
			Required:    true,
			Description: "IPPool Metadata.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:        schema.TypeString,
						Optional:    true,
						ForceNew:    true,
						Computed:    true,
						Description: "Name is the name of the IPPool.",
					},
					"resource_version": {
						Type:        schema.TypeString,
						Description: "An opaque value that represents the internal version",
						Computed:    true,
					},
					"annotations": {
						Type:         schema.TypeMap,
						Description:  "An unstructured key value map",
						Optional:     true,
						Elem:         &schema.Schema{Type: schema.TypeString},
						ValidateFunc: validateAnnotations,
					},
				},
			},
		},
		"spec": {
			Type:        schema.TypeList,
			Description: "Spec defines the specification of the desired behavior of the IPPool. More info: https://projectcalico.docs.tigera.io/reference/resources/ippool",
			Required:    true,
			MaxItems:    1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"cidr": {
						Type:     schema.TypeString,
						Required: true,
						ForceNew: true,
					},
					"block_size": {
						Type:        schema.TypeInt,
						ForceNew:    true,
						Optional:    true,
						Default:     defaultAttributes["block_size"],
						Description: "The CIDR size of allocation blocks used by this pool. Blocks are allocated on demand to hosts and are used to aggregate routes. The value can only be set when the pool is created.",
					},
					"ipip_mode": {
						Type:          schema.TypeString,
						Optional:      true,
						Default:       defaultAttributes["ipip_mode"],
						Description:   "The mode defining when IPIP will be used. Cannot be set at the same time as vxlanMode.",
						ValidateFunc:  validation.StringInSlice([]string{"Always", "CrossSubnet", "Never"}, false),
						ConflictsWith: []string{"spec.0.vxlan_mode"},
					},
					"vxlan_mode": {
						Type:          schema.TypeString,
						Optional:      true,
						Default:       defaultAttributes["vxlan_mode"],
						Description:   "The mode defining when VXLAN will be used. Cannot be set at the same time as ipipMode.",
						ValidateFunc:  validation.StringInSlice([]string{"Always", "CrossSubnet", "Never"}, false),
						ConflictsWith: []string{"spec.0.ipip_mode"},
					},
					"nat_outgoing": {
						Type:        schema.TypeBool,
						Optional:    true,
						Default:     defaultAttributes["nat_outgoing"],
						Description: "When enabled, packets sent from Calico networked containers in this pool to destinations outside of this pool will be masqueraded.",
					},
					"disabled": {
						Type:        schema.TypeBool,
						Optional:    true,
						Default:     defaultAttributes["disabled"],
						Description: "When set to true, Calico IPAM will not assign addresses from this pool.",
					},
					"disable_bgp_export": {
						Type:        schema.TypeBool,
						Optional:    true,
						Default:     defaultAttributes["disable_bgp_export"],
						Description: "Disable exporting routes from this IP Pool’s CIDR over BGP.",
					},
				},
			},
		},
	}
}

func getIPPool(ctx context.Context, m *Meta, p *clientset.Clientset, name string) (*calicov3.IPPool, error) {
	debug("%s getIPPool wait for lock", name)
	m.Lock()
	defer m.Unlock()
	debug("%s getIPPool got lock, started", name)

	r, err := p.ProjectcalicoV3().IPPools().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		debug("getIPPool for %s errored", name)
		return nil, err
	}

	debug("%s getIPPool done", name)

	return r, nil
}

func setResourceAttributes(d *schema.ResourceData, r *calicov3.IPPool) error {
	d.SetId(r.Name)

	metadata := []map[string]interface{}{{"name": r.Name, "resource_version": r.ResourceVersion, "annotations": r.Annotations}}
	if err := d.Set("metadata", metadata); err != nil {
		return err
	}

	spec := []map[string]interface{}{{
		"block_size":         r.Spec.BlockSize,
		"cidr":               r.Spec.CIDR,
		"disabled":           r.Spec.Disabled,
		"ipip_mode":          r.Spec.IPIPMode,
		"vxlan_mode":         r.Spec.VXLANMode,
		"nat_outgoing":       r.Spec.NATOutgoing,
		"disable_bgp_export": r.Spec.DisableBGPExport,
	}}
	if err := d.Set("spec", spec); err != nil {
		return err
	}
	return nil
}

func expandStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v.(string)
	}
	return result
}

func setIPPoolAttributes(d *schema.ResourceData, r *calicov3.IPPool) error {
	spec := calicov3.IPPoolSpec{}
	spec.BlockSize = d.Get("spec.0.block_size").(int)
	spec.CIDR = d.Get("spec.0.cidr").(string)
	spec.Disabled = d.Get("spec.0.disabled").(bool)
	spec.IPIPMode = getIPIPMode(d.Get("spec.0.ipip_mode").(string))
	spec.VXLANMode = getVXLANMode(d.Get("spec.0.vxlan_mode").(string))
	spec.NATOutgoing = d.Get("spec.0.nat_outgoing").(bool)
	spec.DisableBGPExport = d.Get("spec.0.disable_bgp_export").(bool)

	r.Name = d.Get("metadata.0.name").(string)
	r.ResourceVersion = d.Get("metadata.0.resource_version").(string)
	r.Annotations = expandStringMap(d.Get("metadata.0.annotations").(map[string]interface{}))
	// expandStringMap(m["annotations"])
	r.Spec = spec

	return nil
}

func resourceIPPoolExists(ctx context.Context, d *schema.ResourceData, meta interface{}) (bool, error) {
	logID := fmt.Sprintf("[resourceIPPoolExists: %s]", d.Get("metadata.0.name").(string))
	debug("%s Start", logID)

	name := d.Get("metadata.0.name").(string)

	m := meta.(*Meta)

	p, err := m.GetCalicoConfiguration()
	if err != nil {
		return false, err
	}

	_, err = getIPPool(ctx, m, p, name)

	// fixme try to get not fount

	debug("%s Done", logID)

	if err == nil {
		return true, nil
	}

	return false, err
}

func resourceIPPoolRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	exists, err := resourceIPPoolExists(ctx, d, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	if !exists {
		d.SetId("")
		return diag.Diagnostics{}
	}

	logID := fmt.Sprintf("[resourceIPPoolRead: %s]", d.Get("metadata.0.name").(string))
	debug("%s Started", logID)

	m := meta.(*Meta)

	p, err := m.GetCalicoConfiguration()
	if err != nil {
		return diag.FromErr(err)
	}

	name := d.Get("metadata.0.name").(string)
	r, err := getIPPool(ctx, m, p, name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = setResourceAttributes(d, r)
	if err != nil {
		return diag.FromErr(err)
	}

	debug("%s Done", logID)

	return nil
}

func getVXLANMode(mode string) calicov3.VXLANMode {
	switch mode {
	case "Always":
		return calicov3.VXLANModeAlways
	case "CrossSubnet":
		return calicov3.VXLANModeCrossSubnet
	}
	return calicov3.VXLANModeNever
}

func getIPIPMode(mode string) calicov3.IPIPMode {
	switch mode {
	case "Always":
		return calicov3.IPIPModeAlways
	case "CrossSubnet":
		return calicov3.IPIPModeCrossSubnet
	}
	return calicov3.IPIPModeNever
}

func resourceIPPoolCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	m := meta.(*Meta)

	p, err := m.GetCalicoConfiguration()
	if err != nil {
		return diag.FromErr(err)
	}

	ippool := calicov3.IPPool{}
	err = setIPPoolAttributes(d, &ippool)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = p.ProjectcalicoV3().IPPools().Create(ctx, &ippool, metav1.CreateOptions{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(ippool.Name)

	return nil
}

func resourceIPPoolUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	m := meta.(*Meta)

	p, err := m.GetCalicoConfiguration()
	if err != nil {
		return diag.FromErr(err)
	}

	ippool := calicov3.IPPool{}
	err = setIPPoolAttributes(d, &ippool)
	if err != nil {
		return diag.FromErr(err)
	}

	_, err = p.ProjectcalicoV3().IPPools().Update(ctx, &ippool, metav1.UpdateOptions{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(ippool.Name)

	return nil
}

func resourceIPPoolDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	name := d.Get("metadata.0.name").(string)

	m := meta.(*Meta)

	p, err := m.GetCalicoConfiguration()
	if err != nil {
		return diag.FromErr(err)
	}

	err = p.ProjectcalicoV3().IPPools().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

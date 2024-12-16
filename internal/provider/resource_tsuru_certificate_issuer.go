// Copyright 2024 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package provider

import (
	"context"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tsuru/go-tsuruclient/pkg/tsuru"
)

func resourceTsuruCertificateIssuer() *schema.Resource {
	return &schema.Resource{
		Description:   "Set a issuer to generate certificates to a tsuru application",
		CreateContext: resourceTsuruCertificateIssuerSet,
		ReadContext:   resourceTsuruCertificateIssuerRead,
		DeleteContext: resourceTsuruCertificateIssuerUnset,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},
		Importer: &schema.ResourceImporter{
			StateContext: resourceTsuruApplicationImport,
		},
		Schema: map[string]*schema.Schema{
			"app": {
				Type:        schema.TypeString,
				Description: "Application name",
				Required:    true,
				ForceNew:    true,
			},

			"cname": {
				Type:        schema.TypeString,
				Description: "Application CNAME",
				Required:    true,
				ForceNew:    true,
			},

			"issuer": {
				Type:        schema.TypeString,
				Description: "Certificate Issuer",
				Required:    true,
				ForceNew:    true,
			},

			"router": {
				Type:        schema.TypeList,
				Description: "Routers that are using the certificate",
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"certificate": {
				Type:        schema.TypeList,
				Description: "Certificate Generated by Issuer, filled after the certificate is ready",
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"ready": {
				Type:        schema.TypeBool,
				Description: "If the certificate is ready",
				Computed:    true,
			},
		},
	}
}

func resourceTsuruCertificateIssuerSet(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	provider := meta.(*tsuruProvider)

	app := d.Get("app").(string)
	cname := d.Get("cname").(string)
	issuer := d.Get("issuer").(string)

	_, err := provider.TsuruClient.AppApi.AppSetCertIssuer(context.Background(), app, tsuru.CertIssuerSetData{
		Cname:  cname,
		Issuer: issuer,
	})

	if err != nil {
		return diag.Errorf("unable to set certificate issuer: %v", err)
	}

	d.SetId(app + "::" + cname + "::" + issuer)

	return resourceTsuruCertificateIssuerRead(ctx, d, meta)
}

func resourceTsuruCertificateIssuerUnset(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	provider := meta.(*tsuruProvider)
	parts, err := IDtoParts(d.Id(), 3)
	if err != nil {
		return diag.FromErr(err)
	}
	app := parts[0]
	cname := parts[1]

	_, err = provider.TsuruClient.AppApi.AppUnsetCertIssuer(context.Background(), app, cname)

	if err != nil {
		return diag.Errorf("unable to unset certificate issuer: %v", err)
	}

	return resourceTsuruCertificateIssuerRead(ctx, d, meta)
}

func resourceTsuruCertificateIssuerRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	provider := meta.(*tsuruProvider)
	parts, err := IDtoParts(d.Id(), 3)
	if err != nil {
		return diag.FromErr(err)
	}
	app := parts[0]
	cname := parts[1]
	issuer := parts[2]

	certificates, _, err := provider.TsuruClient.AppApi.AppGetCertificates(context.Background(), app)
	if err != nil {
		return diag.FromErr(err)
	}

	usedRouters := []string{}
	usedCertificates := []string{}

	for routerName, router := range certificates.Routers {
		cnameInRouter, ok := router.Cnames[cname]
		if !ok {
			continue
		}

		if cnameInRouter.Issuer != issuer {
			continue
		}
		usedRouters = append(usedRouters, routerName)

		if cnameInRouter.Certificate != "" {
			usedCertificates = append(usedCertificates, cnameInRouter.Certificate)
		}
	}

	sort.Strings(usedRouters)
	sort.Strings(usedCertificates)

	d.Set("app", app)
	d.Set("cname", cname)
	d.Set("issuer", issuer)

	d.Set("router", usedRouters)
	d.Set("certificate", usedCertificates)
	d.Set("ready", len(usedCertificates) > 0)

	return nil
}
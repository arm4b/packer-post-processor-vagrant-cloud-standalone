// vagrant_cloud implements the packer.PostProcessor interface and adds a
// post-processor that uploads artifacts from the vagrant post-processor
// to Vagrant Cloud (vagrantcloud.com) or manages self hosted boxes on the
// Vagrant Cloud
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/template/interpolate"
)

const VAGRANT_CLOUD_URL = "https://vagrantcloud.com/api/v1"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	Tag                string `mapstructure:"box_tag"`
	Version            string `mapstructure:"version"`
	VersionDescription string `mapstructure:"version_description"`
	NoRelease          bool   `mapstructure:"no_release"`

	AccessToken     string `mapstructure:"access_token"`
	VagrantCloudUrl string `mapstructure:"vagrant_cloud_url"`

	BoxDownloadUrl string `mapstructure:"box_download_url"`

	// Target provider name like 'virtualbox'
	ProviderName string `mapstructure:"provider"`
	// Local artifact file path to upload
	ArtifactFile string `mapstructure:"artifact"`

	ctx interpolate.Context
}

type boxDownloadUrlTemplate struct {
	ArtifactId string
	Provider   string
}

type PostProcessor struct {
	config         Config
	client         *VagrantCloudClient
	runner         multistep.Runner
	warnAtlasToken bool
}

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"box_download_url",
			},
		},
	}, raws...)
	if err != nil {
		return err
	}

	// Default configuration
	if p.config.VagrantCloudUrl == "" {
		p.config.VagrantCloudUrl = VAGRANT_CLOUD_URL
	}

	if p.config.AccessToken == "" {
		envToken := os.Getenv("VAGRANT_CLOUD_TOKEN")
		if envToken == "" {
			envToken = os.Getenv("ATLAS_TOKEN")
			if envToken != "" {
				p.warnAtlasToken = true
			}
		}
		p.config.AccessToken = envToken
	}

	// Accumulate any errors
	errs := new(packer.MultiError)

	// required configuration
	templates := map[string]*string{
		"box_tag":       &p.config.Tag,
		"version":       &p.config.Version,
		"access_token":  &p.config.AccessToken,
		"provider_name": &p.config.ProviderName,
		"artifact":      &p.config.ArtifactFile,
	}

	for key, ptr := range templates {
		if *ptr == "" {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("%s must be set", key))
		}
	}

	if len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *PostProcessor) PostProcess(ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, error) {
	// We assume that there is only one .box file to upload
	if !strings.HasSuffix(p.config.ArtifactFile, ".box") {
		return nil, false, fmt.Errorf(
			"Unknown artifact file specified, expected '.box', got: %s", p.config.ArtifactFile)
	}

	if !fileExists(p.config.ArtifactFile) {
		return nil, false, fmt.Errorf(
			"Artifact file specified doesn't exist: %s", p.config.ArtifactFile)
	}

	if p.warnAtlasToken {
		ui.Message("Warning: Using Vagrant Cloud token found in ATLAS_TOKEN. Please make sure it is correct, or set VAGRANT_CLOUD_TOKEN")
	}

	// create the HTTP client
	p.client = VagrantCloudClient{}.New(p.config.VagrantCloudUrl, p.config.AccessToken)

	p.config.ctx.Data = &boxDownloadUrlTemplate{
		ArtifactId: artifact.Id(),
		Provider:   p.config.ProviderName,
	}
	boxDownloadUrl, err := interpolate.Render(p.config.BoxDownloadUrl, &p.config.ctx)
	if err != nil {
		return nil, false, fmt.Errorf("Error processing box_download_url: %s", err)
	}

	// Set up the state
	state := new(multistep.BasicStateBag)
	state.Put("config", p.config)
	state.Put("client", p.client)
	state.Put("artifact", artifact)
	state.Put("artifactFilePath", p.config.ArtifactFile)
	state.Put("ui", ui)
	state.Put("providerName", p.config.ProviderName)
	state.Put("boxDownloadUrl", boxDownloadUrl)

	// Build the steps
	steps := []multistep.Step{}
	if p.config.BoxDownloadUrl == "" {
		steps = []multistep.Step{
			new(stepVerifyBox),
			new(stepCreateVersion),
			new(stepCreateProvider),
			new(stepPrepareUpload),
			new(stepUpload),
			new(stepReleaseVersion),
		}
	} else {
		steps = []multistep.Step{
			new(stepVerifyBox),
			new(stepCreateVersion),
			new(stepCreateProvider),
			new(stepReleaseVersion),
		}
	}

	// Run the steps
	p.runner = common.NewRunner(steps, p.config.PackerConfig, ui)
	p.runner.Run(state)

	// If there was an error, return that
	if rawErr, ok := state.GetOk("error"); ok {
		return nil, false, rawErr.(error)
	}

	return NewArtifact(p.config.ProviderName, p.config.Tag), true, nil
}

// Runs a cleanup if the post processor fails to upload
func (p *PostProcessor) Cancel() {
	if p.runner != nil {
		log.Println("Cancelling the step runner...")
		p.runner.Cancel()
	}
}

// Check if target file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err != nil
}

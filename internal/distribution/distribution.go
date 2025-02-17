package distribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

var DistributionNotFound = errors.New("Distribution not available")

type DistributionItem struct {
	Description string `json:"description"`
	Name        string `json:"name"`
}

type Distributions []DistributionItem

type DistributionFile struct {
	ModulePlatformID string           `json:"module_platform_id"`
	Distribution     DistributionItem `json:"distribution"`
	ArchX86          *Architecture    `json:"x86_64,omitempty"`
}

type Architecture struct {
	ImageTypes   []string     `json:"image_types"`
	Repositories []Repository `json:"repositories"`
}

type Repository struct {
	Id            string   `json:"id"`
	Baseurl       string   `json:"baseurl"`
	Rhsm          bool     `json:"rhsm"`
	ImageTypeTags []string `json:"image_type_tags"`
}

type Package struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type PackagesFile struct {
	Data []Package `json:"data"`
}

func allDistributions(distsDir string) ([]string, error) {
	files, err := ioutil.ReadDir(distsDir)
	if err != nil {
		return nil, err
	}
	var ds []string
	for _, f := range files {
		ds = append(ds, f.Name())
	}
	return ds, nil
}

func validDistribution(distsDir, distro string) (string, error) {
	allDistros, err := allDistributions(distsDir)
	if err != nil {
		return "", err
	}

	for _, d := range allDistros {
		if distro == d {
			return d, nil
		}
	}
	return "", DistributionNotFound
}

func ReadDistribution(distsDir, distroIn string) (d DistributionFile, err error) {
	distro, err := validDistribution(distsDir, distroIn)
	if err != nil {
		return
	}

	f, err := os.Open(filepath.Clean(path.Join(distsDir, distro, fmt.Sprintf("%s.json", distro))))
	if err != nil {
		return
	}
	// nosec because of https://github.com/securego/gosec/issues/714
	/* #nosec G307 */
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("Error closing file: %v", err)
		}
	}()
	err = json.NewDecoder(f).Decode(&d)
	return
}

func AvailableDistributions(distsDir string) (Distributions, error) {
	allDistros, err := allDistributions(distsDir)
	if err != nil {
		return nil, err
	}

	var distros Distributions
	for _, f := range allDistros {
		df, err := ReadDistribution(distsDir, f)
		if err != nil {
			return nil, err
		}
		distros = append(distros, df.Distribution)
	}
	return distros, nil
}

func RepositoriesForArch(distsDir, distro, arch string) ([]Repository, error) {
	distribution, err := ReadDistribution(distsDir, distro)
	if err != nil {
		return nil, err
	}

	switch arch {
	case "x86_64":
		return distribution.ArchX86.Repositories, nil
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Architecture not supported")
	}
}

func FindPackages(distsDir, distro, arch, search string) ([]Package, error) {
	repos, err := RepositoriesForArch(distsDir, distro, arch)
	if err != nil {
		return nil, err
	}

	var pkgs []Package
	for _, r := range repos {
		// Ignore repositories that do not apply to all for now
		if len(r.ImageTypeTags) > 0 {
			continue
		}
		f, err := os.Open(filepath.Clean(path.Join(distsDir, distro, fmt.Sprintf("%s-%s-%s-packages.json", distro, arch, r.Id))))
		if err != nil {
			return nil, err
		}

		var ps []Package
		err = json.NewDecoder(f).Decode(&ps)
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			if strings.Contains(p.Name, search) {
				pkgs = append(pkgs, p)
			}
		}
	}
	return pkgs, nil
}

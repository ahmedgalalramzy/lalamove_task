package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-github/github"
)

// ReadData reads data and returns owners, repositories and minVersions
func ReadData(textFile string) ([]string, []string, []string) {

	var owners []string       // slice to store owners
	var repositories []string // slice to store repositories
	var minVersions []string  // slice to store minimum Version

	Data, err := os.Open(textFile) // Read data from the file
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(Data) // scan through the data
	defer Data.Close()
	i := 0
	for scanner.Scan() {
		if i == 0 { // skip header
			i++
			continue
		}
		err := scanner.Err()
		if err != nil {
			println(err)
			continue
		}
		line := scanner.Text()                        // each line in the file
		parts := strings.Split(line, "/")             // store owners and the rest (repository and minimum version) to a string slice called parts
		owners = append(owners, parts[0])             //update owners with the new owner
		parts = strings.Split(parts[1], ",")          // store repository and minimum version to parts
		repositories = append(repositories, parts[0]) //update repositories slice with the new repository
		minVersions = append(minVersions, parts[1])   // update minimum versions slice wih the new minimum version
	}

	return owners, repositories, minVersions
}

// LatestVersions returns a sorted slice with the highest version as its first element and the highest version of the smaller minor versions in a descending order
func LatestVersions(releases []*semver.Version, minVersion *semver.Version) []*semver.Version {

	var versionSlice []*semver.Version

	for _, version := range releases { // iterate through the releases

		if version != nil && version.PreRelease == "" && version.Compare(*minVersion) >= 0 { // if the version should be added to the slice

			i := len(versionSlice) - 1 //iterator to determine the location of insertion
			for i >= 0 && version.Major > versionSlice[i].Major {
				i-- //decrement until the first position or an older major version is found
			}
			if i < 0 || version.Major < versionSlice[i].Major { // if the versions with that major don't already exist
				versionSlice = append(versionSlice, version) // create a space for it at the end
				copy(versionSlice[i+2:], versionSlice[i+1:]) // shift older versions to the right of the location
				versionSlice[i+1] = version                  //insert after the first version older than the version
			} else { // version has the same major as versionSlice[i]
				for i >= 0 && version.Minor > versionSlice[i].Minor {
					i-- //decrement until the first position or an older minor version is found
				}
				if i < 0 || version.Minor < versionSlice[i].Minor { // if the minor is different, add version to the slice
					versionSlice = append(versionSlice, version) // create a space for it at the end
					copy(versionSlice[i+2:], versionSlice[i+1:]) // shift older versions to the right of the location
					versionSlice[i+1] = version
				} else if version.Patch > versionSlice[i].Patch { //if version has the same minor as versionSlice[i], keep the one with higher patch
					versionSlice[i] = version
				}
			}
		}
	}
	return versionSlice
}

func main() {
	textfile := os.Args[1] //reading the file name
	client := github.NewClient(nil)
	ctx := context.Background()
	owners, repositories, minVersions := ReadData(textfile) // get the data from the file
	for i := range owners {
		opt := &github.ListOptions{PerPage: 10}
		var AllofReleases []*semver.Version // slice to store releases from different pages
		minVersion := semver.New(minVersions[i])
		done := false // done == true means we moved to the page where versions are older than miVersion
		for {
			releases, resp, err := client.Repositories.ListReleases(ctx, owners[i], repositories[i], opt) // resp is declared to iterate through pages
			if err != nil {
				fmt.Println(err)
				continue // move to the next page if there is an error
			}
			allReleases := make([]*semver.Version, len(releases))
			for j, release := range releases {
				versionString := *release.TagName
				if versionString[0] < '1' { // Major version 0 is for initial development and is not stable. 1.0.0 is the public release
					continue
				}
				if versionString[0] == 'v' {
					versionString = versionString[1:]
				}
				allReleases[j], err = semver.NewVersion(versionString) // if there is an error parsing the string
				if err != nil {
					continue // ignore that release (allReleases[i] == nil)
				}
			}
			lastVersionOnPage := allReleases[len(allReleases)-1]
			if minVersion.Compare(*lastVersionOnPage) > 0 { // the last version on the page is older than minVersions, then the following pages won't have version that would be included
				done = true
			}
			AllofReleases = append(AllofReleases, allReleases...) // add the releases from this page to the previous ones
			if resp.NextPage == 0 || done {                       // if there isn't a next page
				break
			}
			opt.Page = resp.NextPage // move to next page
		}
		versionSlice := LatestVersions(AllofReleases, minVersion)
		fmt.Printf("latest versions of %s/%s: %s\n", owners[i], repositories[i], versionSlice)

	}
}

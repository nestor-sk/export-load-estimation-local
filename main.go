package main

import (
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	tool         = "/Applications/Sketch Experimental.app/Contents/MacOS/sketchtool"
	exportList   = "export presentation --formats=list"
	exportMarina = "export presentation --formats=marina"
	data         = "data"
	docs         = "docs"
	numberOfRuns = 5
)

type result struct {
	user   int64
	system int64
	memory int64
}

type cmdResults map[string][]result //kind is key

var sheet = map[string][]cmdResults{} //file is key

func listDocuments(path string) []string {
	path, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err)
	}

	var files []string
	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		if strings.HasSuffix(fileName, ".sketch") {
			files = append(files, filepath.Join(path, fileName))
		}
	}

	return files
}

func execute(cmd string) result {
	fmt.Printf("Executing: %s\n", cmd)
	cloudCmd := exec.Command("bash", "-c", cmd)
	cloudCmd.Start()

	err := cloudCmd.Wait()
	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	}
	usage := cloudCmd.ProcessState.SysUsage().(*syscall.Rusage)

	return result{user: cloudCmd.ProcessState.UserTime().Milliseconds(), system: cloudCmd.ProcessState.SystemTime().Milliseconds(), memory: usage.Maxrss / 1000 / 1000}
}

func processSheet(baseKind string) {
	//PRINT
	fmt.Printf("\n\nRESULTS\n=======\n")
	fmt.Println("User\tMem\tFile")
	for file, runs := range sheet {
		var baseAvgResult result
		var candidateAvgResult result
		for _, run := range runs {
			for kind, results := range run {
				var sumResult = result{}
				for _, result := range results {
					sumResult.user += result.user
					sumResult.system += result.system
					sumResult.memory += result.memory
				}
				numOfResults := int64(len(results))
				var avgResult = result{sumResult.user / numOfResults, sumResult.system / numOfResults, sumResult.memory / numOfResults}
				if kind == baseKind {
					baseAvgResult = avgResult
				} else {
					candidateAvgResult = avgResult
				}
			}
		}
		userTimeDiff := (baseAvgResult.user - candidateAvgResult.user) * 100 / baseAvgResult.user
		memDiff := (baseAvgResult.memory - candidateAvgResult.memory) * 100 / baseAvgResult.memory
		fmt.Printf("%v%%\t%v%%\t%v\n", userTimeDiff, memDiff, file)
	}

	//CSV
	dataOutput, err := os.Create(filepath.Join(data, "results.csv"))
	if err != nil {
		fmt.Printf("Failed to create output file %v", err)
		return
	}
	defer dataOutput.Close()

	dataOutputWriter := csv.NewWriter(dataOutput)
	dataOutputWriter.Write([]string{"document", "operation", "user_time_ms", "system_time_ms", "memory_mb"})
	defer dataOutputWriter.Flush()

	for file, runs := range sheet {
		for _, run := range runs {
			for kind, results := range run {
				for _, result := range results {
					dataOutputWriter.Write([]string{
						file,
						kind,
						fmt.Sprint(result.user),
						fmt.Sprint(result.system),
						fmt.Sprint(result.memory),
					})
				}
			}
		}
	}
}

func main() {

	for _, file := range listDocuments(docs) {
		var runs = []cmdResults{}
		for _, kind := range []string{exportList, exportMarina} {
			var results = []result{}
			for i := 0; i < numberOfRuns; i++ {
				outputDir, err := ioutil.TempDir(os.TempDir(), "load_test-")
				if err != nil {
					fmt.Printf("Failed to create temp dir: %v\n", err)
				}

				cmd := fmt.Sprintf("%q %s %q --output=%q", tool, kind, file, outputDir)
				res := execute(cmd)

				results = append(results, res)

				os.RemoveAll(outputDir)
			}
			runs = append(runs, cmdResults{kind: results})
		}
		sheet[file] = runs
	}

	processSheet(exportList)

}

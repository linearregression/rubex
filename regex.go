package rubex

/*
#cgo LDFLAGS: -L/usr/local/lib -lonig
#cgo CFLAGS: -I/usr/local/include
#include <stdlib.h>
#include <oniguruma.h>
#include "chelper.h"
*/
import "C"

import (
  "unsafe"
  "fmt"
  "os"
)

type strRange []int
const numMatchStartSize = 4
const numCapturePerMatchStartSize = 4

type MatchData struct {
  //captures[0] gives the beginning and ending index of this match
  //captures[j] (j >=1) gives the beginning and ending index of the j-th capture for this match
  captures []strRange
  //namedCaptures["foo"] gives the j index of named capture "foo", then j can be used to get the beginning and ending index of the capture for this match
  namedCaptures map[string]int
}

type Regexp struct {
  pattern string
  regex C.OnigRegex
  region *C.OnigRegion
  encoding C.OnigEncoding
  errorInfo *C.OnigErrorInfo
  errorBuf *C.char
  //matchData[i-1] is the i-th match -- there could be multiple non-overlapping matches for a given pattern
  matchData []*MatchData
}

func NewRegexp(pattern string, option int) (re *Regexp, err os.Error) {
  re = &Regexp{pattern: pattern}
  error_code := C.NewOnigRegex(C.CString(pattern), C.int(len(pattern)), C.int(option), &re.regex, &re.region, &re.encoding, &re.errorInfo, &re.errorBuf)
  if error_code != C.ONIG_NORMAL {
    fmt.Printf("error: %q\n", C.GoString(re.errorBuf))
    err = os.NewError(C.GoString(re.errorBuf))
  } else {
    err = nil
    re.matchData = make([]*MatchData, 0, numMatchStartSize)
  }
  return re, err
}

func Compile(str string) (*Regexp, os.Error) {
  return NewRegexp(str, ONIG_OPTION_DEFAULT)
}

func MustCompile(str string) *Regexp {
  regexp, error := NewRegexp(str, ONIG_OPTION_DEFAULT) 
  if error != nil {
    panic("regexp: compiling " + str + ": " + error.String())
  }
  return regexp
}

func (re *Regexp) Free() {
  if re.regex != nil {
    C.onig_free(re.regex)
    re.regex = nil
  }
  if re.region != nil {
    C.onig_region_free(re.region, 1)
  }
  if re.errorInfo != nil {
    C.free(unsafe.Pointer(re.errorInfo))
    re.errorInfo = nil
  }
  if re.errorBuf != nil {
    C.free(unsafe.Pointer(re.errorBuf))
    re.errorBuf = nil
  }
}

func (re *Regexp) GetCaptureAt(at int) (sr strRange) {
  sr = nil
  if len(re.matchData) > 0 && at < len(re.matchData[0].captures) {
    sr = re.matchData[0].captures[at]
  }
  return
}

func (re *Regexp) GetCaptures()(srs []strRange) {
  srs = nil
  if len(re.matchData) > 0 {
    srs = re.matchData[0].captures
  }
  return
}

func (re *Regexp) GetAllCaptures()(srs [][]strRange) {
  srs = make([][]strRange, 0, numMatchStartSize)
  if len(re.matchData) > 0 {
    fmt.Printf("number of matches: %d\n", len(re.matchData))
    for i:= 0; i < len(re.matchData); i ++ {
      srs = append(srs, re.matchData[i].captures)
    }
  }
  return
}

func (re *Regexp) getStrRange(ref int) (sr strRange) {
  sr = make([]int, 2)
  sr[0] = int(C.IntAt(re.region.beg, C.int(ref)))
  sr[1] = int(C.IntAt(re.region.end, C.int(ref)))
  return 
}

func (re *Regexp) processMatch() (sr strRange) {
  matchData := &MatchData{}
  matchData.captures = make([]strRange, 0, 1+numCapturePerMatchStartSize) //the first element is not really a capture
  num := (int (re.region.num_regs))
  fmt.Printf("processMatch %d\n", num)
  for i := 0; i < num; i ++ {
    sr := re.getStrRange(i)
    matchData.captures = append(matchData.captures, sr)
  }
  re.matchData = append(re.matchData, matchData)
  return matchData.captures[0]
}

func (re *Regexp) find(b []byte, n int) (pos int, err os.Error) {
  ptr := unsafe.Pointer(&b[0])
  pos = int(C.SearchOnigRegex((ptr), C.int(n), C.int(ONIG_OPTION_DEFAULT), re.regex, re.region, re.encoding, re.errorInfo, re.errorBuf))
  if pos >= 0 {
    err = nil
  } else {
    err = os.NewError(C.GoString(re.errorBuf))
  }
  return pos, err
}

func (re *Regexp) findAll(b []byte, n int, deliver func(sr strRange)) (err os.Error) {
  offset := 0
  bp := b[offset:]
  _, err = re.find(bp, n - offset)
  if err == nil {
    sr := re.processMatch()
    sr[0] += offset
    sr[1] += offset
    deliver(sr)
    offset = sr[1]
  }
  for ;err == nil && offset < n; {
    bp = b[offset:]
    _, err = re.find(bp, n - offset)
    if err == nil {
      sr := re.processMatch()
      sr[0] += offset
      sr[1] += offset
      deliver(sr)
      offset = sr[1]
    }
  }
  return err
}

func (re *Regexp) Find(b []byte) []byte {
  _, err := re.find(b, len(b))
  if err == nil {
    sr := re.processMatch()
    return b[sr[0]:sr[1]]
  }
  return nil  
 
}

func (re *Regexp) FindIndex(b []byte) (loc []int) {
  _, err := re.find(b, len(b))
  if err == nil {
    return re.processMatch()
  }
  return nil
}

func (re *Regexp) FindString(s string) string {
  b := []byte(s)
  mb := re.Find(b)
  if mb == nil {
    return ""
  }
  return string(mb)
}

func (re *Regexp) FindStringIndex(s string) []int {
  b := []byte(s)
  return re.FindIndex(b)
}

func (re *Regexp) FindAll(b []byte, n int) [][]byte {
  results := make([][]byte, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    results = append(results, b[sr[0]:sr[1]])
  })
  if len(results) == 0 {
    return nil  
  }
  return results
}

func (re *Regexp) FindAllString(s string, n int) []string {
  b := []byte(s)
  results := make([]string, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    results = append(results, string(b[sr[0]:sr[1]]))
  })
  
  if len(results) == 0 {
    return nil  
  }
  return results
}

func (re *Regexp) FindAllIndex(b []byte, n int) [][]int { 
  results := make([][]int, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    m := make([]int,2)
    m[0] = sr[0]
    m[1] = sr[1]
    results = append(results, m)
  })
  if len(results) == 0 {
    return nil  
  }
  return results
}

func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
  b := []byte(s)
  return re.FindAllIndex(b, n)
}

func (re *Regexp) FindSubmatch(b []byte) [][]byte {
  _, err := re.find(b, len(b))
  results := make([][]byte, 0, numCapturePerMatchStartSize)
  if err == nil {  
    captures := re.GetCaptures()
    fmt.Printf("%v\n", captures)
    for _, cap := range captures {
      results = append(results, b[cap[0]:cap[1]])
    }
  }
  if len(results) == 0 {
    return nil
  }
  return results
}

func (re *Regexp) FindSubmatchIndex(b []byte) []int {
  _, err := re.find(b, len(b))
  results := make([]int, 0, numCapturePerMatchStartSize*2)
  if err == nil {
    captures := re.GetCaptures()
    fmt.Printf("%v\n", captures)
    for _, cap := range captures {
      results = append(results, cap[0])
      results = append(results, cap[1])
    }
  }
  if len(results) == 0 {
    return nil
  }
  return results
}

func (re *Regexp) FindStringSubmatch(s string) []string {
  b := []byte(s)
  _, err := re.find(b, len(b))
  results := make([]string, 0, numCapturePerMatchStartSize)
  if err == nil {
    captures := re.GetCaptures()
    fmt.Printf("%v\n", captures)
    for _, cap := range captures {
      results = append(results, string(b[cap[0]:cap[1]]))
    }
  }
  if len(results) == 0 {
    return nil
  }
  return results 
}

func (re *Regexp) FindStringSubmatchIndex(s string) []int {
  b := []byte(s)
  return re.FindSubmatchIndex(b)  
}

func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
  results := make([][][]byte, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    fmt.Printf("%d %d\n", sr[0], sr[1])
  })
  allCaptures := re.GetAllCaptures()
  fmt.Printf("%v\n", allCaptures)
  for _, captures := range allCaptures {
    r := make([][]byte, 0, numCapturePerMatchStartSize)
    for _, cap := range captures {
      r = append(r, b[cap[0]:cap[1]])
    }
    results = append(results, r)
  }
  
  if len(results) == 0 {
    return nil
  }
  return results
}

func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
  b := []byte(s)
  results := make([][]string, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    fmt.Printf("%d %d\n", sr[0], sr[1])
  })
  allCaptures := re.GetAllCaptures()
  fmt.Printf("%v\n", allCaptures)
  for _, captures := range allCaptures {
    r := make([]string, 0, numCapturePerMatchStartSize)
    for _, cap := range captures {
      r = append(r, string(b[cap[0]:cap[1]]))
    }
    results = append(results, r)
  }
  
  if len(results) == 0 {
    return nil
  }
  return results

  return nil
}

func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
  results := make([][]int, 0, numMatchStartSize)
  re.findAll(b, n, func(sr strRange) {
    fmt.Printf("%d %d\n", sr[0], sr[1])
  })
  allCaptures := re.GetAllCaptures()
  fmt.Printf("%v\n", allCaptures)
  for _, captures := range allCaptures {
    r := make([]int, 0, numCapturePerMatchStartSize*2)
    for _, cap := range captures {
      r = append(r, cap[0])
      r = append(r, cap[1])
    }
    results = append(results, r)
  }
  
  if len(results) == 0 {
    return nil
  }
  return results

}

func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
  b := []byte(s)
  return re.FindAllSubmatchIndex(b, n)
}

func (re *Regexp) Match(b []byte) bool {
  _, err := re.find(b, len(b))
  return err == nil
}

func (re *Regexp) MatchString(s string) bool {
  b := []byte(s)
  return re.Match(b)
}

func (re *Regexp) NumSubexp() int {
  return (int)(C.onig_number_of_captures(re.regex))
}

func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
  newSrc := make([]byte, 0, len(src))
  sr := re.FindIndex(src)
  fmt.Printf("sr: %v\n", sr)
  if sr == nil {
    for _, c := range src {
      newSrc = append(newSrc, c)
    }
    return newSrc  
  }
  fmt.Printf("sr   : %v\n", sr)
  newRepl := make([]byte, 0, len(repl) * 3)
  inEscapeMode := false
  for _, ch := range repl {
    fmt.Printf("ch: xx %v %v\n", string(ch), inEscapeMode)
    
    if inEscapeMode && ch <= byte('9') && byte('1') <= ch {
      fmt.Printf("ch yy: %v\n", string(ch))
      capNum := int(ch - byte('0'))
      fmt.Printf("capNum %v\n", capNum)
      sr1 := re.GetCaptureAt(capNum)
      fmt.Printf("sr1: %v\n", sr1)
      if sr1 == nil {
        panic(fmt.Sprintf("cannot find the capture: \\%d\n", capNum))
      }
      for _, c := range src[sr1[0]:sr1[1]] {
        newRepl = append(newRepl, c)
      }
    } else if inEscapeMode {
      newRepl = append(newRepl, '\\')
      newRepl = append(newRepl, ch)
    } else if ch != '\\' {
      newRepl = append(newRepl, ch)
    }
    if ch == byte('\\') || inEscapeMode {
      inEscapeMode = !inEscapeMode
    }
  }
  fmt.Printf("newRepl: %v\n", string(newRepl))
  newSrc = make([]byte, 0, len(src) * 2)
  if sr[0] > 0 {
    for _,c := range src[0:sr[0]-1] {
      newSrc = append(newSrc, c)
    }
  }
  for _,c := range newRepl {
    newSrc = append(newSrc, c)
  }
  if sr[1] < len(src) {
    for _, c:= range src[sr[1]:] {
      newSrc = append(newSrc, c)
    }
  }

  fmt.Printf("newSrc: %v\n", string(newSrc))
  return newSrc
}

func (re *Regexp) String() string {
  return re.pattern
}

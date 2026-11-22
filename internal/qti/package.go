// Package qti writes the portable QTI 3.0 subset used by SDT assessment banks.
// It intentionally supports only choice-based interactions; broader QTI belongs
// to a maintained QTI implementation rather than this writer.
package qti

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	qtiNamespace                 = "http://www.imsglobal.org/xsd/imsqtiasi_v3p0"
	manifestNS                   = "http://www.imsglobal.org/xsd/qti/qtiv3p0/imscp_v1p1"
	itemResourceType             = "imsqti_item_xmlv3p0"
	interactionSingleChoice      = "single-choice"
	interactionMultipleChoice    = "multiple-choice"
	interactionTrueFalse         = "true-false"
	interactionChoiceExplanation = "choice-explanation"
	scoringExactSet              = "exact-set"
	scoringPartialCredit         = "partial-credit"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

type Option struct {
	ID   string
	Text string
}

type Item struct {
	ID                string
	Title             string
	Prompt            string
	Objective         int
	Claim             string
	Provenance        string
	Interaction       string
	Options           []Option
	Correct           []string
	Scoring           string
	ExplanationPrompt string
}

type Package struct {
	ID    string
	Title string
	Items []Item
}

// WritePackage returns a deterministic IMS QTI 3.0 item-bank ZIP.
func WritePackage(input Package) ([]byte, error) {
	if !identifierPattern.MatchString(input.ID) {
		return nil, fmt.Errorf("invalid package identifier %q", input.ID)
	}
	if len(input.Items) == 0 {
		return nil, errors.New("QTI package has no items")
	}
	files := make(map[string][]byte, len(input.Items)+1)
	files["imsmanifest.xml"] = []byte(renderManifest(input))
	seen := map[string]bool{}
	for _, item := range input.Items {
		if seen[item.ID] {
			return nil, fmt.Errorf("duplicate item identifier %q", item.ID)
		}
		seen[item.ID] = true
		xmlData, err := renderItem(item)
		if err != nil {
			return nil, err
		}
		files["items/"+item.ID+".xml"] = []byte(xmlData)
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		header := &zip.FileHeader{Name: path, Method: zip.Deflate}
		header.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[path]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func renderItem(item Item) (string, error) {
	if item.Scoring == "" {
		item.Scoring = scoringExactSet
	}
	if err := validateItem(item); err != nil {
		return "", err
	}
	var b strings.Builder
	write(&b, `<?xml version="1.0" encoding="UTF-8"?>`)
	write(&b, `<qti-assessment-item xmlns="`+qtiNamespace+`" identifier="`+escAttr(item.ID)+`" title="`+escAttr(item.Title)+`" adaptive="false" time-dependent="false">`)
	writeResponseDeclaration(&b, item)
	if item.Interaction == interactionChoiceExplanation {
		write(&b, `<qti-response-declaration identifier="EXPLANATION" cardinality="single" base-type="string"/>`)
	}
	write(&b, `<qti-outcome-declaration identifier="SCORE" cardinality="single" base-type="float"/>`)
	writeItemBody(&b, item)
	writeResponseProcessing(&b, item)
	write(&b, `</qti-assessment-item>`)
	return b.String(), nil
}

func validateItem(item Item) error {
	if !identifierPattern.MatchString(item.ID) {
		return fmt.Errorf("invalid item identifier %q", item.ID)
	}
	if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Prompt) == "" {
		return fmt.Errorf("item %q requires title and prompt", item.ID)
	}
	if !supportedInteraction(item.Interaction) {
		return fmt.Errorf("item %q has unsupported interaction %q", item.ID, item.Interaction)
	}
	if len(item.Options) < 2 || len(item.Correct) == 0 {
		return fmt.Errorf("item %q requires options and correct option ids", item.ID)
	}
	if (item.Interaction == interactionSingleChoice || item.Interaction == interactionTrueFalse || item.Interaction == interactionChoiceExplanation) && len(item.Correct) != 1 {
		return fmt.Errorf("item %q requires exactly one correct option", item.ID)
	}
	if item.Interaction == interactionTrueFalse && len(item.Options) != 2 {
		return fmt.Errorf("item %q true-false interaction requires two options", item.ID)
	}
	if item.Interaction == interactionChoiceExplanation && strings.TrimSpace(item.ExplanationPrompt) == "" {
		return fmt.Errorf("item %q requires an explanation prompt", item.ID)
	}
	if item.Scoring != scoringExactSet && item.Scoring != scoringPartialCredit {
		return fmt.Errorf("item %q has unsupported scoring %q", item.ID, item.Scoring)
	}
	if item.Scoring == scoringPartialCredit && item.Interaction != interactionMultipleChoice {
		return fmt.Errorf("item %q partial credit only supports multiple-choice", item.ID)
	}
	optionIDs := map[string]bool{}
	for _, choice := range item.Options {
		if !identifierPattern.MatchString(choice.ID) || strings.TrimSpace(choice.Text) == "" {
			return fmt.Errorf("item %q has invalid option id or empty option text", item.ID)
		}
		if optionIDs[choice.ID] {
			return fmt.Errorf("item %q repeats option id %q", item.ID, choice.ID)
		}
		optionIDs[choice.ID] = true
	}
	correct := map[string]bool{}
	for _, id := range item.Correct {
		if !optionIDs[id] {
			return fmt.Errorf("item %q expected option %q is not declared", item.ID, id)
		}
		if correct[id] {
			return fmt.Errorf("item %q repeats expected option %q", item.ID, id)
		}
		correct[id] = true
	}
	return nil
}

func writeResponseDeclaration(b *strings.Builder, item Item) {
	correct := make(map[string]bool, len(item.Correct))
	for _, id := range item.Correct {
		correct[id] = true
	}
	write(b, `<qti-response-declaration identifier="RESPONSE" cardinality="`+cardinality(item.Interaction)+`" base-type="identifier"><qti-correct-response>`)
	for _, id := range item.Correct {
		write(b, `<qti-value>`+esc(id)+`</qti-value>`)
	}
	write(b, `</qti-correct-response>`)
	if item.Scoring == scoringPartialCredit {
		write(b, `<qti-mapping lower-bound="0" upper-bound="1" default-value="0">`)
		value := fmt.Sprintf("%.12g", 1/float64(len(item.Correct)))
		for _, choice := range item.Options {
			mapped := "-" + value
			if correct[choice.ID] {
				mapped = value
			}
			write(b, `<qti-map-entry map-key="`+esc(choice.ID)+`" mapped-value="`+mapped+`"/>`)
		}
		write(b, `</qti-mapping>`)
	}
	write(b, `</qti-response-declaration>`)
}

func writeItemBody(b *strings.Builder, item Item) {
	write(b, `<qti-item-body><qti-choice-interaction response-identifier="RESPONSE" shuffle="false" max-choices="`+maxChoices(item.Interaction)+`"><qti-prompt>`+esc(item.Prompt)+`</qti-prompt>`)
	for _, choice := range item.Options {
		write(b, `<qti-simple-choice identifier="`+escAttr(choice.ID)+`">`+esc(choice.Text)+`</qti-simple-choice>`)
	}
	write(b, `</qti-choice-interaction>`)
	if item.Interaction == interactionChoiceExplanation {
		write(b, `<qti-extended-text-interaction response-identifier="EXPLANATION"><qti-prompt>`+esc(item.ExplanationPrompt)+`</qti-prompt></qti-extended-text-interaction>`)
	}
	write(b, `</qti-item-body>`)
}

func writeResponseProcessing(b *strings.Builder, item Item) {
	if item.Scoring == scoringPartialCredit {
		write(b, `<qti-response-processing template="https://www.imsglobal.org/question/qti_v3p0/rptemplates/map_response.xml"/>`)
	} else if item.Interaction == interactionChoiceExplanation {
		write(b, `<qti-response-processing><qti-response-condition><qti-response-if><qti-match><qti-variable identifier="RESPONSE"/><qti-correct identifier="RESPONSE"/></qti-match><qti-set-outcome-value identifier="SCORE"><qti-base-value base-type="float">1</qti-base-value></qti-set-outcome-value></qti-response-if><qti-response-else><qti-set-outcome-value identifier="SCORE"><qti-base-value base-type="float">0</qti-base-value></qti-set-outcome-value></qti-response-else></qti-response-condition></qti-response-processing>`)
	} else {
		write(b, `<qti-response-processing template="https://www.imsglobal.org/question/qti_v3p0/rptemplates/match_correct.xml"/>`)
	}
}

func supportedInteraction(interaction string) bool {
	switch interaction {
	case interactionSingleChoice, interactionMultipleChoice, interactionTrueFalse, interactionChoiceExplanation:
		return true
	default:
		return false
	}
}

func renderManifest(input Package) string {
	var b strings.Builder
	write(&b, `<?xml version="1.0" encoding="UTF-8"?>`)
	write(&b, `<manifest xmlns="`+manifestNS+`" xmlns:lom="http://ltsc.ieee.org/xsd/LOM" identifier="`+esc(input.ID)+`"><metadata><schema>QTI Item Bank</schema><schemaversion>3.0.1</schemaversion>`)
	if input.Title != "" {
		write(&b, `<lom:lom><lom:general><lom:title><lom:string>`+esc(input.Title)+`</lom:string></lom:title></lom:general></lom:lom>`)
	}
	write(&b, `</metadata><organizations/><resources>`)
	for _, item := range input.Items {
		path := "items/" + item.ID + ".xml"
		write(&b, `<resource identifier="`+esc(item.ID)+`" type="`+itemResourceType+`" href="`+path+`"><file href="`+path+`"/>`)
		if item.Objective > 0 || item.Claim != "" || item.Provenance != "" {
			write(&b, `<metadata><lom:lom><lom:general>`)
			if item.Claim != "" {
				write(&b, `<lom:keyword><lom:string>claim:`+esc(item.Claim)+`</lom:string></lom:keyword>`)
			}
			if item.Objective > 0 {
				write(&b, `<lom:keyword><lom:string>objective:`+fmt.Sprint(item.Objective)+`</lom:string></lom:keyword>`)
			}
			if item.Provenance != "" {
				write(&b, `<lom:keyword><lom:string>provenance:`+esc(item.Provenance)+`</lom:string></lom:keyword>`)
			}
			write(&b, `</lom:general></lom:lom></metadata>`)
		}
		write(&b, `</resource>`)
	}
	write(&b, `</resources></manifest>`)
	return b.String()
}

func cardinality(interaction string) string {
	if interaction == interactionMultipleChoice {
		return "multiple"
	}
	return "single"
}

func maxChoices(interaction string) string {
	if interaction == interactionMultipleChoice {
		return "0"
	}
	return "1"
}

func esc(value string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		panic(err) // bytes.Buffer cannot fail
	}
	return b.String()
}

func escAttr(value string) string {
	return strings.ReplaceAll(esc(value), `"`, `&#34;`)
}

func write(out io.StringWriter, value string) {
	if _, err := out.WriteString(value); err != nil {
		panic(err) // current callers use bytes.Buffer or strings.Builder
	}
}

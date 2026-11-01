#!/bin/sh
# Measures what the references answer for a resource whose number RFC 8259
# refuses (#238), which the difftest harness cannot carry: its batch is JSON.
# ra-NN.json holds line NN+1 of values.txt as a RiskAssessment's
# probabilityDecimal. fhirpath.js takes a parsed object, so what reaches it is
# what JSON.parse makes of the text; the HL7 validator reads the file itself.
#
# Usage, from the repository root: difftest/batteries/resource-number-grammar/refused/measure.sh
# HL7_VALIDATOR_JAR defaults to build/validator/validator_cli-6.10.2.jar (make validator-jar).
set -eu
dir=$(dirname "$0")
jar=${HL7_VALIDATOR_JAR:-build/validator/validator_cli-6.10.2.jar}
expr='RiskAssessment.prediction.probability + 1'
for file in "$dir"/ra-*.json; do
	parsed=$(node -e 'try { JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); console.log("parsed") } catch (e) { console.log(e.name) }' "$file")
	java_out=$(java -Xmx3g -jar "$jar" -fhirpath "$expr" "$file" -version 4.0.1 -tx n/a 2>&1 |
		sed 's/\x1b\[[0-9;]*m//g' | awk 'found && NF { print; exit } /evaluating/ { found = 1 }')
	printf '%s\tJSON.parse: %s\tvalidator: %s\n' "$(basename "$file")" "$parsed" "$java_out"
done

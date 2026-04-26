// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"strings"
	"testing"
)

func TestParsePOM_Minimal(t *testing.T) {
	xml := `<project>
  <groupId>com.example</groupId>
  <artifactId>app</artifactId>
  <version>1.0.0</version>
</project>`

	pom, err := ParsePOM(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pom.GroupID != "com.example" || pom.ArtifactID != "app" || pom.Version != "1.0.0" {
		t.Fatalf("unexpected pom: %+v", pom)
	}
}

func TestParsePOM_Full(t *testing.T) {
	xml := `<project>
  <groupId>com.example</groupId>
  <artifactId>app</artifactId>
  <version>2.0.0</version>
  <packaging>jar</packaging>
  <name>My App</name>
  <description>A test app</description>
  <url>https://example.com</url>
  <licenses>
    <license>
      <name>Apache 2.0</name>
      <url>https://www.apache.org/licenses/LICENSE-2.0</url>
    </license>
  </licenses>
  <dependencies>
    <dependency>
      <groupId>org.slf4j</groupId>
      <artifactId>slf4j-api</artifactId>
      <version>1.7.36</version>
      <scope>compile</scope>
    </dependency>
  </dependencies>
</project>`

	pom, err := ParsePOM(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pom.Licenses) != 1 || pom.Licenses[0].Name != "Apache 2.0" {
		t.Errorf("unexpected licenses: %+v", pom.Licenses)
	}

	if len(pom.Dependencies) != 1 || pom.Dependencies[0].GroupID != "org.slf4j" {
		t.Errorf("unexpected dependencies: %+v", pom.Dependencies)
	}

	if pom.Packaging != "jar" || pom.Name != "My App" {
		t.Errorf("unexpected fields: packaging=%q name=%q", pom.Packaging, pom.Name)
	}
}

func TestParsePOM_MalformedXML(t *testing.T) {
	_, err := ParsePOM(strings.NewReader(`<project><groupId>bad`))
	if err == nil {
		t.Fatal("expected error for malformed XML")
	}
}

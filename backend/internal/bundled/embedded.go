//go:build embedweb

package bundled

import _ "embed"

//go:embed runtime.tar.gz
var archive []byte

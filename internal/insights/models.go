package insights

import "github.com/we-promise/sure-cli/internal/models"

// Transaction is re-exported for backwards compatibility within the insights package.
// Prefer using internal/models.Transaction when outside of insights.
type Transaction = models.Transaction

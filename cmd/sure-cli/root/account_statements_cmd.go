package root

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"

	"github.com/we-promise/sure-cli/internal/api"
)

const accountStatementsPath = "/api/v1/account_statements"

// accountStatementMaxBytes mirrors AccountStatement::MAX_FILE_SIZE (25 MB).
const accountStatementMaxBytes = 25 << 20

// accountStatementExtensions mirrors AccountStatement::ACCEPTED_FILE_EXTENSIONS.
var accountStatementExtensions = []string{".pdf", ".csv", ".xlsx"}

// accountStatementReviewStatuses mirrors the AccountStatement review_status enum.
var accountStatementReviewStatuses = []string{"unmatched", "linked", "rejected"}

type accountStatementListOpts struct {
	AccountID     string
	ReviewStatus  string
	ContentSHA256 string
	StartDate     string
	EndDate       string
	Page          int
	PerPage       int
}

type accountStatementUploadOpts struct {
	File      string
	AccountID string
	Apply     bool
}

type accountStatementUpload struct {
	upload
	ByteSize int64
}

func newAccountStatementsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "account-statements", Short: "Statement Vault documents (PDF/CSV/XLSX)"}

	var lo accountStatementListOpts
	list := &cobra.Command{
		Use:   "list",
		Short: "List statements visible to the caller (newest first)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q, err := buildAccountStatementListQuery(lo)
			failValidation(err)
			printGet(pathWithQuery(accountStatementsPath, q))
		},
	}
	addPagingFlags(list, &lo.Page, &lo.PerPage)
	list.Flags().StringVar(&lo.AccountID, "account-id", "", "statements linked to this account")
	list.Flags().StringVar(&lo.ReviewStatus, "review-status", "", "unmatched|linked|rejected")
	list.Flags().StringVar(&lo.ContentSHA256, "content-sha256", "", "the statement for a file, by its SHA-256 (hex)")
	list.Flags().StringVar(&lo.StartDate, "start-date", "", "statements whose period ends on or after this date (YYYY-MM-DD)")
	list.Flags().StringVar(&lo.EndDate, "end-date", "", "statements whose period starts on or before this date (YYYY-MM-DD)")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Show a statement (data.meta.duplicate is always false here)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printGet(fmt.Sprintf("%s/%s", accountStatementsPath, url.PathEscape(args[0])))
		},
	})

	cmd.AddCommand(newAccountStatementsUploadCmd())
	return cmd
}

func newAccountStatementsUploadCmd() *cobra.Command {
	var o accountStatementUploadOpts
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a statement to the vault (default dry-run; use --apply to execute)",
		Long: `Upload a .pdf, .csv or .xlsx statement (at most 25 MB) to the Statement Vault.
Archives the document only; it never imports transactions (use imports for that).

Responses: 201 stored; 200 the same file is already in the vault (data.meta.duplicate true);
409 duplicate linked to an account you cannot see; 404 account-id not writable by you.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			u, err := buildAccountStatementUpload(o)
			failValidation(err)
			if !o.Apply {
				printDryRun("POST", accountStatementsPath, u.dryRunBody())
				return
			}
			var res any
			r, err := postAccountStatement(api.New(), u, &res)
			respond(r, err, res)
		},
	}
	cmd.Flags().StringVar(&o.File, "file", "", "path to the statement (.pdf, .csv or .xlsx, max 25 MB)")
	cmd.Flags().StringVar(&o.AccountID, "account-id", "", "link to this account (needs write access); omit to file it in the vault inbox")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the upload (otherwise dry-run)")
	return cmd
}

func buildAccountStatementListQuery(o accountStatementListOpts) (url.Values, error) {
	if !oneOfOrEmpty(o.ReviewStatus, accountStatementReviewStatuses...) {
		return nil, fmt.Errorf("--review-status must be one of: %s", strings.Join(accountStatementReviewStatuses, ", "))
	}
	if _, _, err := parseOptionalDateRange(o.StartDate, o.EndDate); err != nil {
		return nil, err
	}

	q := url.Values{}
	addPagingQuery(q, o.Page, o.PerPage)
	for k, v := range map[string]string{
		"account_id":     o.AccountID,
		"review_status":  o.ReviewStatus,
		"content_sha256": o.ContentSHA256,
		"start_date":     o.StartDate,
		"end_date":       o.EndDate,
	} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q, nil
}

// buildAccountStatementUpload validates the file client-side against the
// same rules AccountStatement.prepare_upload! applies (extension allow-list,
// non-empty, at most 25 MB) so obviously bad uploads never leave the machine.
func buildAccountStatementUpload(o accountStatementUploadOpts) (accountStatementUpload, error) {
	if o.File == "" {
		return accountStatementUpload{}, errors.New("--file is required")
	}
	ext := strings.ToLower(filepath.Ext(o.File))
	if !slices.Contains(accountStatementExtensions, ext) {
		return accountStatementUpload{}, fmt.Errorf("--file must be one of: %s", strings.Join(accountStatementExtensions, ", "))
	}
	info, err := regularFile(o.File)
	if err != nil {
		return accountStatementUpload{}, err
	}
	if info.Size() == 0 {
		return accountStatementUpload{}, errors.New("--file is empty")
	}
	if info.Size() > accountStatementMaxBytes {
		return accountStatementUpload{}, fmt.Errorf("--file is %d bytes; the maximum is %d (25 MB)", info.Size(), accountStatementMaxBytes)
	}

	fields := map[string]string{}
	if o.AccountID != "" {
		fields["account_id"] = o.AccountID
	}
	return accountStatementUpload{
		upload:   upload{Fields: fields, FilePath: o.File},
		ByteSize: info.Size(),
	}, nil
}

// dryRunBody adds the part's content type and size to the planned request.
func (u accountStatementUpload) dryRunBody() map[string]any {
	body := u.upload.dryRunBody()
	body["content_type"] = mimeForUploadFile(u.FilePath)
	body["byte_size"] = u.ByteSize
	return body
}

func postAccountStatement(client *api.Client, u accountStatementUpload, out any) (*resty.Response, error) {
	return u.send(client, accountStatementsPath, out)
}

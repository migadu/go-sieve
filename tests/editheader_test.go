package tests

// Note: The Dovecot pigeonhole tests for editheader require additional
// extensions (mailbox, include, body) that are not yet implemented.
// The editheader extension is tested in execute_test.go in the main package.
//
// TODO: Enable these tests once mailbox, include, and body extensions are implemented:
// - TestExtensionsEditheaderAddheader
// - TestExtensionsEditheaderDeleteheader
// - TestExtensionsEditheaderProtected
// - TestExtensionsEditheaderAlternating
// - TestExtensionsEditheaderExecute

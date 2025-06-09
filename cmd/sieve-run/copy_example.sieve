require ["fileinto", "copy"];

# Example of using :copy with fileinto
# This will file the message into the "Reports" folder
# AND continue processing (potentially keeping in INBOX as well)
if header :contains "Subject" "Report" {
    fileinto :copy "Reports";
}

# Example of using :copy with redirect
# This will redirect the message to the specified address
# AND continue processing (potentially keeping in INBOX as well)
if header :contains "Subject" "Forward" {
    redirect :copy "admin@example.com";
}

# No :copy - message will NOT be kept in INBOX
if header :contains "Subject" "NoKeep" {
    redirect "user@example.com";
    # No implicit keep here
}

keep;

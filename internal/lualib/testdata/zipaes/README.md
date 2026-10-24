Zip archives made by 7-Zip (`7zz a -tzip -mem=AES256 -p'sok3r!' ...`), to check
that tlua reads what 7-Zip writes. The password is `sok3r!` (7-Zip refuses a
password that is not ASCII when it makes a zip). Each holds a.txt, text.txt,
rand.bin and empty.txt, whose contents internal/lualib/zipcrypt_test.go
makes again; aes256-store.zip holds text.txt and rand.bin stored, not
deflated.

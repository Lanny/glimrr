# glimrr
GitLab Interactive Merge Request Review


# REMEMBER:
fswatch --exclude ".*\.sw[px]$" --exclude ".*~$" -o ./src | ./watch.sh

curl --header "PRIVATE-TOKEN: $MRAAG_GL_TOKEN" https://gitlab.bstock.io/api/v4/projects/400/merge_requests/530/versions/79964 | python3 -m json.tool

curl --header "PRIVATE-TOKEN: $MRAAG_GL_TOKEN" https://gitlab.bstock.io/api/v4/projects/400/repository/files/src%2Fapp%2Fstore%2Fdata%2Fmodules%2FtaxDoc.js\?ref\=eb41cfd2e052446b4f45024425bd18c75319f2d8 | python3 -m json.tool | jq -r '.content' | base64 --decode > test-data/b

[ryan.jenkins:~/glimrr]$ curl --header "PRIVATE-TOKEN: $MRAAG_GL_TOKEN" https://gitlab.bstock.io/api/v4/projects/400/repository/files/src%2Fapp%2Fstore%2Fdata%2Fmodules%2FtaxDoc.js\?ref\=HEAD | python3 -m json.tool | jq -r '.content' | base64 --decode > test-data/a

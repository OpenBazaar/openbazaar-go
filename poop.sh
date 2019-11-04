    git tag -d test-travis
    git push origin :refs/tags/test-travis
    git add build-osx.sh
    git commit -m "Update shell script"
    git tag -s test-travis -m "test-travis"
    git push origin test-travis

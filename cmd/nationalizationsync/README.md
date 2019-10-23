Nationalizationsync
========

Description
-----------

Nationalizationsync is an utility program: provided a modified [JSON nationalization](https://github.com/negapedia/wikiassignment/tree/master/nationalization/internal/languages), it propagates the new categorical information to other nationalizations using [LangLinks](https://www.mediawiki.org/wiki/Manual:Langlinks_table).

How to use it?
-----------

1. Choose a working dir: this will contain all seed nationalizations, aka the nationalization from which all the others will be derived using [LangLinks](https://www.mediawiki.org/wiki/Manual:Langlinks_table). This folder should contain your modified JSON and all the other that in the past were manually modified, as of now: `en.json`, `es.json`, `fr.json`, `it.json`, `pt.json` and `zh.json`.

2. Run Nationalizationsync on your working dir: `docker run -d -v /path/2/work/dir:/data negapedia/wikiassignment nationalizationsync`. Eventually check progress with `docker logs -f $(docker ps -lq)`. At the end, after some hours, this folder will contain all newly regenerated nationalizations, double check logs and results.

How to integrate these results into the [refresh docker image](https://hub.docker.com/r/negapedia/negapedia)?
-----------

3. [Install golang](https://golang.org/doc/install) and set it properly, or later commands will fail.

4. Install (or update) go-bindata: `go get -u github.com/shuLhan/go-bindata/...` and check that it works (`go-bindata --help`).

5. Install (or update) wikiassignment: `go get -u github.com/negapedia/wikiassignment/`.

6. Find the installation folder and change the working directory into it: `cd /path/2/your/wikiassignment/`.

7. Remove previous nationalizations in [your local languages folder](https://github.com/negapedia/wikiassignment/tree/master/nationalization/internal/languages): `rm nationalization/internal/languages/*`.

8. Add the new nationalizations, generated from step `2`: `mv /path/2/work/dir/* nationalization/internal/languages/`.

9. Change directory into `nationalization/internal` : `cd nationalization/internal/`.

10. Regenerate golang assets from JSON nationalizations: `go generate`. 

11. Include in the git versioning possible new files: `git add -A`.

12. Commit the changes: for ex. `git commit -am "Updated pl nationalization, additional sync seeds: en, es, fr, it, pt and zh"`.

13. Set wikiassignment remote repository: `git remote set-url origin git@github.com:negapedia/wikiassignment.git`.

14. Push the changes: `git push`.

15. Double check the [diff of the changes](https://github.com/negapedia/wikiassignment/commits/master) on Github.

16. Being logged in docker hub, manually [trigger a new build](https://cloud.docker.com/u/negapedia/repository/docker/negapedia/negapedia/builds) for the refresh image, takes a little more than one hour.

17. [Rejoice. ;)](https://www.youtube.com/results?search_query=Monty+Python+-+Dirty+Hungarian+Phrasebook)
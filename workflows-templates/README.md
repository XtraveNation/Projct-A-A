# GitHub Actions templates

Move these into `.github/workflows/` once your token has the `workflow` scope:

```bash
mkdir -p .github/workflows
mv workflows-templates/ci.yml workflows-templates/deploy.yml .github/workflows/
git add .github/workflows && git commit -m "ci: enable workflows" && git push
```

- `ci.yml` — builds & vets on every push.
- `deploy.yml` — SSH-deploys to your VPS on push to `main`.
  Set repo secrets `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, and variable `DEPLOY_ENABLED=true`.

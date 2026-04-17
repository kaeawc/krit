#!/usr/bin/env bash
# Generate a starter krit.yml for a Kotlin project

if [ -f krit.yml ] || [ -f .krit.yml ]; then
    echo "Config already exists. Edit krit.yml or .krit.yml directly."
    exit 0
fi

cat > krit.yml << 'YML'
# Krit configuration
# Docs: https://kaeawc.github.io/krit/configuration/
# Full reference: krit --generate-schema

style:
  MagicNumber:
    excludes: ['**/test/**', '**/*Test.kt', '**/*Spec.kt']
    ignorePropertyDeclaration: true
    ignoreAnnotation: true
    ignoreEnums: true
    ignoreNumbers: ['-1', '0', '1', '2']
  MaxLineLength:
    maxLineLength: 120
    excludeCommentStatements: true
    excludes: ['**/test/**']
  WildcardImport:
    active: true
  ReturnCount:
    max: 3
    excludeGuardClauses: true

complexity:
  LongMethod:
    threshold: 60
  CyclomaticComplexMethod:
    allowedComplexity: 15

naming:
  FunctionNaming:
    ignoreAnnotated:
      - 'Composable'
      - 'Test'

potential-bugs:
  UnsafeCast:
    excludes: ['**/test/**']
YML

echo "Created krit.yml with recommended defaults."
echo "Run 'krit .' to analyze your project."

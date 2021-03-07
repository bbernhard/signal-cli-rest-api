name: Bug Report
about: Something doesn't work as expected.
title: ""
issue_body: true
body:
  - type: textarea
    validations:
      required: true
    attributes:
      label: The problem
      description: >-
        Describe the issue you are experiencing here.

        Provide a clear and concise description of what the problem is.
  - type: markdown
    attributes:
      value: |
        ## Environment
  - type: input
    validations:
      required: true
    attributes:
      label: What version of signal-cli-rest-api has the issue?
      placeholder: 
      description: >
        Can be found in the docker image tag.
  - type: input
    attributes:
      label: What was the last working version signal-cli-rest-api?
      placeholder: core-
      description: >
        If known, otherwise leave blank.
  - type: dropdown
    validations:
      required: true
    attributes:
      label: What type of installation are you running?
      options:
        - Docker
        - Home Assistant Addon
  - type: markdown
    attributes:
      value: |
        # Details

        ```
  - type: textarea
    attributes:
      label: Anything in the logs that might be useful?
      description: For example, error message, or stack traces.
      value: |
        ```txt
        # Put your logs below this line

        ```
  - type: markdown
    attributes:
      value: |
        ## Additional information
  - type: markdown
    attributes:
      value: >
        If you have any additional information, use the field below.
        Please note, you can attach screenshots or screen recordings here, by
        dragging and dropping files in the field below.



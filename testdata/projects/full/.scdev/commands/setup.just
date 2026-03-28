# Setup commands for testing

# Default recipe - runs when no recipe specified
default: all

# Run complete setup
all: install
    @echo "Setup complete for {{env_var('PROJECTNAME')}}"

# Install dependencies
install:
    @echo "Installing dependencies..."

# Show environment variables
env:
    @echo "PROJECTNAME: {{env_var('PROJECTNAME')}}"
    @echo "PROJECTPATH: {{env_var('PROJECTPATH')}}"
    @echo "PROJECTDIR: {{env_var('PROJECTDIR')}}"
    @echo "SCDEV_DOMAIN: {{env_var('SCDEV_DOMAIN')}}"

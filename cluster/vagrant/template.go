package vagrant

var vagrantTemplate = `
CLOUD_CONFIG_PATH = File.join(File.dirname(__FILE__), "{{.CloudConfigPath}}")
SIZE_PATH = File.join(File.dirname(__FILE__), "{{.SizePath}}")
Vagrant.require_version ">= 1.6.0"

size = File.open(SIZE_PATH).read.strip.split(",")
Vagrant.configure(2) do |config|
  config.vm.box = "{{.Box}}"

	config.vm.box_version = "{{.BoxVersion}}"

  config.vm.network "private_network", type: "dhcp"

  ram=(size[0].to_f*1024).to_i
  cpus=size[1]
  config.vm.provider "virtualbox" do |v|
    v.memory = ram
    v.cpus = cpus
  end

  if File.exist?(CLOUD_CONFIG_PATH)
    config.vm.provision "shell", path: "#{CLOUD_CONFIG_PATH}"
  end
end
`

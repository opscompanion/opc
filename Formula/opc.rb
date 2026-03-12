class Opc < Formula
  desc "CLI companion for platform engineers — persistent context, memory, and session management"
  homepage "https://github.com/opscompanion/opc"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/opscompanion/opc/releases/download/v#{version}/opc_darwin_arm64.tar.gz"
    end
    on_intel do
      url "https://github.com/opscompanion/opc/releases/download/v#{version}/opc_darwin_amd64.tar.gz"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/opscompanion/opc/releases/download/v#{version}/opc_linux_arm64.tar.gz"
    end
    on_intel do
      url "https://github.com/opscompanion/opc/releases/download/v#{version}/opc_linux_amd64.tar.gz"
    end
  end

  def install
    bin.install "opc"
  end

  test do
    system "#{bin}/opc", "version"
  end
end

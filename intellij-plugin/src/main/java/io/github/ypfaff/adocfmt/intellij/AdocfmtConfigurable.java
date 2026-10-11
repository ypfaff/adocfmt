package io.github.ypfaff.adocfmt.intellij;

import com.intellij.openapi.fileChooser.FileChooserDescriptorFactory;
import com.intellij.openapi.options.Configurable;
import com.intellij.openapi.project.Project;
import com.intellij.openapi.ui.TextFieldWithBrowseButton;
import com.intellij.ui.components.JBCheckBox;
import com.intellij.util.ui.FormBuilder;
import javax.swing.JComponent;
import javax.swing.JPanel;

/** The page under Tools › adocfmt. */
final class AdocfmtConfigurable implements Configurable {
  private final AdocfmtSettings settings;
  private final AdocfmtIdeSettings ideSettings = AdocfmtIdeSettings.getInstance();
  private final JBCheckBox enabled =
      new JBCheckBox("Format AsciiDoc files in this project with adocfmt");
  private final TextFieldWithBrowseButton binaryPath = new TextFieldWithBrowseButton();

  AdocfmtConfigurable(Project project) {
    settings = AdocfmtSettings.getInstance(project);
    binaryPath.addBrowseFolderListener(
        project, FileChooserDescriptorFactory.createSingleFileNoJarsDescriptor());
  }

  @Override
  public String getDisplayName() {
    return "adocfmt";
  }

  @Override
  public JComponent createComponent() {
    return FormBuilder.createFormBuilder()
        .addComponent(enabled)
        .addLabeledComponent("adocfmt binary for all projects:", binaryPath)
        .addTooltip("Leave empty to run the binary that comes with the plugin.")
        .addComponentFillVertically(new JPanel(), 0)
        .getPanel();
  }

  @Override
  public boolean isModified() {
    return enabled.isSelected() != settings.isEnabled()
        || !binaryPath.getText().equals(ideSettings.getBinaryPath());
  }

  @Override
  public void apply() {
    settings.setEnabled(enabled.isSelected());
    ideSettings.setBinaryPath(binaryPath.getText());
  }

  @Override
  public void reset() {
    enabled.setSelected(settings.isEnabled());
    binaryPath.setText(ideSettings.getBinaryPath());
  }
}

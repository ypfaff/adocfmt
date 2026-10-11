package io.github.ypfaff.adocfmt.intellij;

import com.intellij.ide.util.PropertiesComponent;
import com.intellij.notification.Notification;
import com.intellij.notification.NotificationAction;
import com.intellij.notification.Notifications;
import com.intellij.testFramework.TestActionEvent;
import com.intellij.testFramework.fixtures.BasePlatformTestCase;
import java.util.ArrayList;
import java.util.List;
import org.jetbrains.annotations.NotNull;

public class EnableNotifierTest extends BasePlatformTestCase {
  private final List<Notification> offers = new ArrayList<>();

  @Override
  protected void setUp() throws Exception {
    super.setUp();
    // The light project lives on from test to test, and so do its settings.
    AdocfmtSettings.getInstance(getProject()).setEnabled(false);
    PropertiesComponent.getInstance(getProject()).unsetValue(EnableNotifier.OFFERED);
    getProject()
        .getMessageBus()
        .connect(getTestRootDisposable())
        .subscribe(
            Notifications.TOPIC,
            new Notifications() {
              @Override
              public void notify(@NotNull Notification notification) {
                if (notification.getGroupId().equals("Enable adocfmt")) {
                  offers.add(notification);
                }
              }
            });
  }

  public void testOffersOnAnAsciiDocFile() {
    myFixture.configureByText("test.adoc", "");
    assertEquals(1, offers.size());
  }

  public void testOffersOnlyOnce() {
    myFixture.configureByText("first.adoc", "");
    myFixture.configureByText("second.asciidoc", "");
    assertEquals(1, offers.size());
  }

  public void testOffersNothingWhenSwitchedOn() {
    AdocfmtSettings.getInstance(getProject()).setEnabled(true);
    myFixture.configureByText("test.adoc", "");
    assertEmpty(offers);
  }

  public void testOffersNothingForOtherFiles() {
    myFixture.configureByText("test.txt", "");
    assertEmpty(offers);
  }

  public void testEnableSwitchesItOn() {
    myFixture.configureByText("test.adoc", "");
    Notification offer = offers.get(0);
    NotificationAction enable = (NotificationAction) offer.getActions().get(0);
    enable.actionPerformed(TestActionEvent.createTestEvent(), offer);
    assertTrue(AdocfmtSettings.getInstance(getProject()).isEnabled());
    assertTrue(offer.isExpired());
  }
}
